// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// advance.go is the advance a booked stay asks for. Once ConfirmStay has
// booked the stay (confirm.go), it raises the stay's proforma through the
// finance service -- one line, the rooms at the hold's total, under the room
// tariff's SAC at its GST slab (roomGST) -- issues it, and asks for the
// advance on it with RequestPayment: the advance share of its total
// ([concierge] advance_percent). The payment link goes to the guest's phone
// through the notifier, and back in payment_url.
//
// Paying it is what confirms the stay (finance confirms it), but asking is
// not what books it: with no gateway (RequestPayment is Unimplemented), no
// invoice series, or any failure on the way, the stay stays booked, no link
// is given, and the reply says the team will send the payment details.
//
// A retried ConfirmStay gets the same proforma and the same link: the
// proforma's id and the payment request's id are both the reservation's.

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/google/uuid"
	"github.com/shokki-engineering/aperio/internal/notify"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Biller raises a stay's proforma and asks for its advance: the finance
// service.
type Biller interface {
	CreateInvoice(context.Context, *financepbv1.CreateInvoiceRequest) (*financepbv1.Invoice, error)
	GetInvoice(context.Context, *financepbv1.GetInvoiceRequest) (*financepbv1.Invoice, error)
	IssueInvoice(context.Context, *financepbv1.IssueInvoiceRequest) (*financepbv1.Invoice, error)
	RequestPayment(context.Context, *financepbv1.RequestPaymentRequest) (*financepbv1.RequestPaymentResponse, error)
}

// BillWith raises booked stays' proformas through b and asks for
// advancePercent of each (1 to 100; anything else is 100); without it,
// ConfirmStay asks for no advance and the team sends the details.
func (s *Server) BillWith(b Biller, advancePercent int) {
	if advancePercent <= 0 || advancePercent > 100 {
		advancePercent = 100
	}
	s.biller, s.advancePercent = b, advancePercent
}

// SendLinksThrough sends the advance's payment link to the guest's phone
// through sender; without it the link is only returned.
func (s *Server) SendLinksThrough(sender *notify.Sender) { s.sender = sender }

// AdvanceLinkEvent is the event of the message carrying the payment link.
const AdvanceLinkEvent = "aperio.concierge.stay.advance_requested"

// Room tariff under GST (Notification 11/2017-Central Tax (Rate), heading
// 9963, as amended from 22 Sep 2025): accommodation is SAC 996311, taxed by
// what one room costs a night -- 5% (without ITC) at or under ₹7,500, 18%
// above it.
const (
	roomSAC        = "996311"
	roomSlabRupees = 7500
	roomRateLow    = "5"
	roomRateHigh   = "18"
)

// roomGST is the GST rate, as a percentage, on a room let at perNight
// rupees a night: the hold's total over its rooms and nights.
func roomGST(perNight *big.Rat) string {
	if perNight.Cmp(big.NewRat(roomSlabRupees, 1)) <= 0 {
		return roomRateLow
	}
	return roomRateHigh
}

// requestAdvance raises the stay's proforma and asks for its advance,
// giving the payment link, or "" when there is none to give.
func (s *Server) requestAdvance(ctx context.Context, h held, contact, reservation string, total *money.Money) string {
	if s.biller == nil {
		return ""
	}
	url, err := s.advance(ctx, h, contact, reservation, total)
	if err != nil {
		if status.Code(err) != codes.Unimplemented {
			slog.WarnContext(ctx, "concierge: advance not requested", "reservation", reservation, "err", err)
		}
		return ""
	}
	s.sender.Send(ctx, notify.Message{To: h.Phone, Event: AdvanceLinkEvent, Subject: reservation,
		Text: fmt.Sprintf("Thank you for booking, %s. Pay the advance for your stay here: %s", h.Guest, url)})
	return url
}

// advance is the payment link for the advance on reservation's proforma.
func (s *Server) advance(ctx context.Context, h held, contact, reservation string, total *money.Money) (string, error) {
	pf, err := s.proforma(ctx, h, contact, reservation, total)
	if err != nil {
		return "", err
	}
	req := &financepbv1.RequestPaymentRequest{Name: pf.GetName(),
		RequestId: uuid.NewSHA1(uuid.NameSpaceURL, []byte("aperio:advance:"+reservation)).String()}
	if s.advancePercent < 100 {
		share := new(big.Rat).Mul(pricing.Rat(pf.GetTotal()), big.NewRat(int64(s.advancePercent), 100))
		req.Amount = pricing.Money(pricing.Whole(share), pf.GetTotal().GetCurrencyCode())
	}
	resp, err := s.biller.RequestPayment(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetUrl(), nil
}

// proforma is reservation's issued proforma: made and issued now, or found
// as a retry left it.
func (s *Server) proforma(ctx context.Context, h held, contact, reservation string, total *money.Money) (*financepbv1.Invoice, error) {
	v, err := s.venues.Get(ctx, h.Venue)
	if err != nil {
		return nil, err
	}
	room := h.Space
	if sp, err := s.spaces.Get(ctx, h.Space); err == nil {
		room = sp.GetDisplayName()
	}
	perNight := new(big.Rat).Quo(pricing.Rat(total), big.NewRat(int64(max(h.Rooms, 1))*int64(max(h.Nights, 1)), 1))
	draft := &financepbv1.Invoice{
		Kind: financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA, Contact: contact, Booking: reservation,
		SupplyStateCode: v.GetGst().GetGstStateCode(),
		Lines: []*financepbv1.InvoiceLine{{
			Description: fmt.Sprintf("%s, %s for %s from %d-%02d-%02d", room, count(h.Rooms, "room"), count(h.Nights, "night"),
				h.Arrival[0], h.Arrival[1], h.Arrival[2]),
			Item: h.Space, Quantity: 1, UnitPrice: total, Tax: &typepbv1.Tax{HsnSacCode: roomSAC, GstRate: &decimal.Decimal{Value: roomGST(perNight)}},
		}},
	}
	id := "pf-" + h.Reservation
	inv, err := s.biller.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: h.Venue, InvoiceId: id, Invoice: draft})
	if status.Code(err) == codes.AlreadyExists {
		inv, err = s.biller.GetInvoice(ctx, &financepbv1.GetInvoiceRequest{Name: h.Venue + "/invoices/" + id})
	}
	if err != nil {
		return nil, err
	}
	if inv.GetState() == financepbv1.Invoice_STATE_DRAFT {
		return s.biller.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: inv.GetName()})
	}
	return inv, nil
}
