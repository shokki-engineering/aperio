// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations

// charge.go posts what a done request costs to its booking's bill: one
// invoice line, quantity 1 at the request's charge before tax, appended to the
// booking's draft invoice -- or to a new draft billed to the booking's guest
// when it has none.
//
// The line's item is the request's name, and that is what makes it
// idempotent: before posting, every invoice of the booking that is not
// cancelled is read, and a line already naming the request -- on a draft, an
// issued invoice or a credit note -- means it was charged, so a retried or
// repeated completion never charges twice. A draft written by someone else
// meanwhile fails the etag guard, and the post is tried again from a fresh
// read; a new draft is named after the request, so two at once cannot both
// make one.
//
// The line's tax is the GST of the first dish the request's items name that
// has one, else RestaurantSAC at RestaurantGSTRate.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// The tax of a charge that names no dish with one: SAC 996331, "services
// provided by restaurants, cafes and similar eating facilities including
// takeaway services, room services and door delivery of food", at 5% GST
// (without input tax credit) -- what room service is under, outside a hotel
// whose room tariff makes its restaurant a "specified premises" at 18%. A
// venue that is one gives its dishes their tax in the catalog.
const (
	RestaurantSAC     = "996331"
	RestaurantGSTRate = "5"
)

// postAttempts is how many times a post is tried when another write to the
// booking's invoices got there first.
const postAttempts = 3

// invoiceType is the resource type errors name.
const invoiceType = "aperio.finance.v1/Invoice"

// charge posts r's charge to its booking, when it has both and the server
// can reach finance.
func (h *hooks) charge(ctx context.Context, r *operationspbv1.ServiceRequest) error {
	c := r.GetCharge()
	if h.svcs.Finance == nil || r.GetBooking() == "" || c == nil || (c.GetUnits() == 0 && c.GetNanos() == 0) {
		return nil
	}
	booking := billed(r.GetBooking())
	if booking == "" {
		return servicex.Blocked("NOT_A_BOOKING", requestType, r.GetName(),
			"its booking "+r.GetBooking()+" is not a reservation or an event", "Cancel it and ask again naming the stay or event")
	}
	var err error
	for range postAttempts {
		if err = h.post(ctx, booking, r); !retry(err) {
			return err
		}
	}
	return err
}

// retry reports whether err is another write getting there first.
func retry(err error) bool {
	switch status.Code(err) {
	case codes.Aborted, codes.AlreadyExists:
		return true
	}
	return false
}

// post adds r's line to booking's draft, or to a new one, unless an invoice
// of booking already has it.
func (h *hooks) post(ctx context.Context, booking string, r *operationspbv1.ServiceRequest) error {
	venueName := servicex.ParentOf(r.GetName())
	var draft *financepbv1.Invoice
	token := ""
	for {
		page, err := h.svcs.Finance.ListInvoices(ctx, &financepbv1.ListInvoicesRequest{Parent: venueName, PageSize: 100, PageToken: token,
			Filter: fmt.Sprintf("booking = %q AND state != CANCELLED", booking), OrderBy: "create_time"})
		if err != nil {
			return err
		}
		for _, inv := range page.GetInvoices() {
			for _, l := range inv.GetLines() {
				if l.GetItem() == r.GetName() {
					return nil
				}
			}
			if draft == nil && inv.GetState() == financepbv1.Invoice_STATE_DRAFT && inv.GetKind() == financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE {
				draft = inv
			}
		}
		if token = page.GetNextPageToken(); token == "" {
			break
		}
	}
	line := &financepbv1.InvoiceLine{Description: r.GetSummary(), Item: r.GetName(), Tax: h.taxOf(ctx, r), Quantity: 1, UnitPrice: r.GetCharge()}
	if draft != nil {
		_, err := h.svcs.Finance.UpdateInvoice(ctx, &financepbv1.UpdateInvoiceRequest{
			Invoice:    &financepbv1.Invoice{Name: draft.GetName(), Etag: draft.GetEtag(), Lines: append(draft.GetLines(), line)},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"lines"}},
		})
		return err
	}
	contact, err := h.contactOf(ctx, booking)
	if err != nil {
		return err
	}
	if contact == "" {
		return servicex.Blocked("NO_CONTACT", invoiceType, booking, "the booking names no guest to bill", "Set the booking's contact, then complete it again")
	}
	v, err := h.venues.Get(ctx, venueName)
	if err != nil {
		return servicex.NotFoundAs(err, venueType, venueName)
	}
	_, err = h.svcs.Finance.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: venueName, InvoiceId: draftID(r.GetName()),
		Invoice: &financepbv1.Invoice{Kind: financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE, Contact: contact, Booking: booking,
			SupplyStateCode: v.GetGst().GetGstStateCode(), Lines: []*financepbv1.InvoiceLine{line}}})
	return err
}

// draftID is the id of the draft a request's charge starts: "charges-" and
// the request's id, at most 63 characters, lower-case.
func draftID(request string) string {
	id := strings.ToLower("charges-" + request[strings.LastIndex(request, "/")+1:])
	return strings.TrimRight(id[:min(len(id), 63)], "-")
}

// taxOf is the GST of r's line: the first dish of its items with a tax,
// else RestaurantSAC at RestaurantGSTRate. A dish that cannot be read is
// passed over.
func (h *hooks) taxOf(ctx context.Context, r *operationspbv1.ServiceRequest) *typepbv1.Tax {
	for _, it := range r.GetItems() {
		if it.GetDish() == "" || h.svcs.Dishes == nil {
			continue
		}
		d, err := h.svcs.Dishes.GetDish(ctx, &catalogpbv1.GetDishRequest{Name: it.GetDish()})
		if err == nil && d.GetTax().GetHsnSacCode() != "" && d.GetTax().GetGstRate().GetValue() != "" {
			return d.GetTax()
		}
	}
	return &typepbv1.Tax{HsnSacCode: RestaurantSAC, GstRate: &decimal.Decimal{Value: RestaurantGSTRate}}
}
