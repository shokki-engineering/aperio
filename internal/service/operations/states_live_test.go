// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations_test

// A live test of the state methods over the dev database: a request
// acknowledged, started and completed, with its timestamps; an illegal move
// and a stale etag refused; a cancel keeping its reason; the department
// told when a request is made and the guest when it is done; and a charged
// request posted to its stay's draft invoice exactly once, whether completed
// again or charged again.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/service/operations/ -run States -v

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/notify"
	"github.com/shokki-engineering/aperio/internal/service/operations"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// recorder is a Notifier that keeps what it is sent.
type recorder struct {
	mu   sync.Mutex
	sent []notify.Message
}

func (r *recorder) Notify(_ context.Context, m notify.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return nil
}

func TestLiveStates(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := &recorder{}
	sender := notify.NewSender(rec, time.Second, nil, nil)
	if err := f.svc.NotifyThrough(sender, map[string]string{"pantry": "+919800000001", "default": "+919800000009"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.NotifyThrough(sender, map[string]string{"spa": "+1"}); err == nil {
		t.Error("a route for no department was taken")
	}

	ask := func(r *operationspbv1.ServiceRequest) *operationspbv1.ServiceRequest {
		t.Helper()
		out, err := f.svc.CreateServiceRequest(ctx, &operationspbv1.CreateServiceRequestRequest{Parent: f.venue, ServiceRequest: r})
		f.must(err, "create "+r.GetSummary())
		return out
	}
	coffee := ask(&operationspbv1.ServiceRequest{Summary: "2 × black coffee to 204", Department: pantry, Location: "204",
		Items: []*operationspbv1.RequestItem{{DisplayName: "Black coffee", Quantity: 2, Notes: "no sugar"}}, Requester: f.contact})
	towels := ask(&operationspbv1.ServiceRequest{Summary: "Towels", Department: housekeep, Location: "204"})
	sender.Wait()
	if len(rec.sent) != 2 || rec.sent[0].To != "+919800000001" || rec.sent[0].Subject != coffee.GetName() ||
		rec.sent[0].Text != "2 × black coffee to 204 -- 2 × Black coffee (no sugar)." || rec.sent[1].To != "+919800000009" {
		t.Fatalf("on create, sent %+v", rec.sent)
	}

	// OPEN -> ACKNOWLEDGED -> IN_PROGRESS -> DONE.
	if _, err := f.svc.StartServiceRequest(ctx, &operationspbv1.StartServiceRequestRequest{Name: coffee.GetName()}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("start an open request: %v, want FAILED_PRECONDITION", err)
	}
	if _, err := f.svc.AcknowledgeServiceRequest(ctx, &operationspbv1.AcknowledgeServiceRequestRequest{Name: coffee.GetName(), Etag: "stale"}); status.Code(err) != codes.Aborted {
		t.Errorf("acknowledge with a stale etag: %v, want ABORTED", err)
	}
	ack, err := f.svc.AcknowledgeServiceRequest(ctx, &operationspbv1.AcknowledgeServiceRequestRequest{Name: coffee.GetName(), Etag: coffee.GetEtag()})
	f.must(err, "acknowledge")
	if ack.GetState() != operationspbv1.ServiceRequest_STATE_ACKNOWLEDGED || ack.GetAcknowledgeTime() == nil {
		t.Errorf("acknowledged = %v at %v", ack.GetState(), ack.GetAcknowledgeTime())
	}
	started, err := f.svc.StartServiceRequest(ctx, &operationspbv1.StartServiceRequestRequest{Name: coffee.GetName()})
	f.must(err, "start")
	if started.GetState() != operationspbv1.ServiceRequest_STATE_IN_PROGRESS {
		t.Errorf("started = %v", started.GetState())
	}
	finished, err := f.svc.CompleteServiceRequest(ctx, &operationspbv1.CompleteServiceRequestRequest{Name: coffee.GetName()})
	f.must(err, "complete")
	if finished.GetState() != operationspbv1.ServiceRequest_STATE_DONE || finished.GetCompleteTime() == nil {
		t.Errorf("completed = %v at %v", finished.GetState(), finished.GetCompleteTime())
	}
	sender.Wait()
	if last := rec.sent[len(rec.sent)-1]; len(rec.sent) != 3 || last.To != f.phone || last.Event != operations.CompletedEvent {
		t.Errorf("on complete, sent %+v; want the guest told on %s", rec.sent, f.phone)
	}
	if _, err := f.svc.CancelServiceRequest(ctx, &operationspbv1.CancelServiceRequestRequest{Name: coffee.GetName()}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("cancel a done request: %v, want FAILED_PRECONDITION", err)
	}

	// Cancelled with a reason, kept in the notes; final.
	off, err := f.svc.CancelServiceRequest(ctx, &operationspbv1.CancelServiceRequestRequest{Name: towels.GetName(), Reason: "guest checked out"})
	f.must(err, "cancel")
	if off.GetState() != operationspbv1.ServiceRequest_STATE_CANCELLED || off.GetNotes() != "Cancelled: guest checked out" {
		t.Errorf("cancelled = %v, notes %q", off.GetState(), off.GetNotes())
	}
	if _, err := f.svc.AcknowledgeServiceRequest(ctx, &operationspbv1.AcknowledgeServiceRequestRequest{Name: towels.GetName()}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("acknowledge a cancelled request: %v, want FAILED_PRECONDITION", err)
	}

	// A charged request to the stay: on a new draft, once; the second on the
	// same draft, under the dish's tax.
	inr := func(u int64) *money.Money { return &money.Money{CurrencyCode: "INR", Units: u} }
	if _, err := f.svc.CreateServiceRequest(ctx, &operationspbv1.CreateServiceRequestRequest{Parent: f.venue, ServiceRequest: &operationspbv1.ServiceRequest{
		Summary: "Elsewhere", Department: pantry, Booking: "venues/another/reservations/x"}}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("a booking of another venue: %v, want INVALID_ARGUMENT", err)
	}
	done := func(r *operationspbv1.ServiceRequest) *operationspbv1.ServiceRequest {
		t.Helper()
		_, err := f.svc.AcknowledgeServiceRequest(ctx, &operationspbv1.AcknowledgeServiceRequestRequest{Name: r.GetName()})
		f.must(err, "acknowledge "+r.GetSummary())
		out, err := f.svc.CompleteServiceRequest(ctx, &operationspbv1.CompleteServiceRequestRequest{Name: r.GetName()})
		f.must(err, "complete "+r.GetSummary())
		return out
	}
	tea := done(ask(&operationspbv1.ServiceRequest{Summary: "Tea for two", Department: pantry, Booking: f.booking, Charge: inr(200)}))
	cake := done(ask(&operationspbv1.ServiceRequest{Summary: "Midnight cake", Department: kitchen, Booking: f.booking, Charge: inr(1000),
		Items: []*operationspbv1.RequestItem{{Dish: f.dish, Quantity: 1}}}))
	if _, err := f.svc.CompleteServiceRequest(ctx, &operationspbv1.CompleteServiceRequestRequest{Name: tea.GetName()}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("complete twice: %v, want FAILED_PRECONDITION", err)
	}
	f.chargeAgain(t, tea) // a retried charge of a request already on the bill

	invoices, err := f.money.ListInvoices(ctx, &financepbv1.ListInvoicesRequest{Parent: f.venue, Filter: `booking = "` + f.booking + `"`})
	f.must(err, "list invoices")
	if len(invoices.GetInvoices()) != 1 {
		t.Fatalf("invoices of the stay = %d, want 1", len(invoices.GetInvoices()))
	}
	inv := invoices.GetInvoices()[0]
	lines := inv.GetLines()
	if inv.GetState() != financepbv1.Invoice_STATE_DRAFT || inv.GetContact() != f.contact || len(lines) != 2 ||
		lines[0].GetItem() != tea.GetName() || lines[0].GetTax().GetHsnSacCode() != operations.RestaurantSAC ||
		lines[0].GetTax().GetGstRate().GetValue() != operations.RestaurantGSTRate || lines[0].GetUnitPrice().GetUnits() != 200 ||
		lines[1].GetItem() != cake.GetName() || lines[1].GetTax().GetHsnSacCode() != "996334" || lines[1].GetTax().GetGstRate().GetValue() != "18" {
		t.Errorf("draft = %v, %s, lines %v", inv.GetState(), inv.GetContact(), lines)
	}
	// 200 at 5% and 1000 at 18%: 1200 + 10 + 180.
	if inv.GetTotal().GetUnits() != 1390 {
		t.Errorf("total = %v, want 1390", inv.GetTotal())
	}
}
