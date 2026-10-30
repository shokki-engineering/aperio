// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge_test

// Live tests of the advance a booked stay asks for: with a gateway (a fake
// payment linker behind the real finance service), ConfirmStay raises the
// stay's proforma, asks for the advance and sends the link -- the same
// proforma and link when retried; without one, or with no billing at all,
// the stay is booked all the same and the reply says the team will send
// the details. Run as test/live says.

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/notify"
	"github.com/shokki-engineering/aperio/internal/service/concierge"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
)

// invoicePrefix is a venue's invoice prefix of this run: four characters,
// so a number fits GST's sixteen.
func invoicePrefix() string {
	p := strings.ToUpper(strconv.FormatInt(time.Now().UnixNano(), 36))
	return p[len(p)-4:]
}

// fakeLinker makes one payment link per reference, as a gateway does.
type fakeLinker struct {
	mu    sync.Mutex
	links map[string]finance.Link
}

func (*fakeLinker) Name() string { return "fakepay" }
func (f *fakeLinker) Link(_ context.Context, r finance.LinkRequest) (finance.Link, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if l, ok := f.links[r.Reference]; ok {
		return l, nil
	}
	l := finance.Link{ID: "plink_" + r.Reference, URL: "https://pay.example/" + r.Reference, Amount: r.Amount, Expire: r.Expire}
	f.links[r.Reference] = l
	return l, nil
}

// inbox records the messages sent.
type inbox struct {
	mu  sync.Mutex
	got []notify.Message
}

func (i *inbox) Notify(_ context.Context, m notify.Message) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.got = append(i.got, m)
	return nil
}

func TestLiveConfirmStayAdvance(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	res := newResort(t)
	v := res.venue
	v.InvoicePrefix = invoicePrefix()
	if _, err := res.venues.Venues.Update(ctx, v, []string{"invoice_prefix"}); err != nil {
		t.Fatalf("set invoice prefix: %v", err)
	}
	book := func(month int32) *conciergepbv1.ConfirmStayResponse {
		t.Helper()
		held, err := res.svc.HoldStay(ctx, &conciergepbv1.HoldStayRequest{Venue: v.GetName(), Space: res.space.GetName(), Nights: 2, Adults: 2,
			ArrivalDate: &date.Date{Year: int32(time.Now().Year()) + 1, Month: month, Day: 3}, GuestDisplayName: "John Doe",
			Phone: "98765 " + res.run[len(res.run)-5:], RequestId: uuid.NewString()})
		if err != nil {
			t.Fatalf("hold: %v", err)
		}
		req := &conciergepbv1.ConfirmStayRequest{Venue: v.GetName(), HoldToken: held.GetHoldToken(), Confirmed: true}
		booked, err := res.svc.ConfirmStay(ctx, req)
		if err != nil {
			t.Fatalf("confirm: %v", err)
		}
		return booked
	}
	honest := func(what string, booked *conciergepbv1.ConfirmStayResponse) {
		t.Helper()
		if booked.GetReservation() == "" || booked.GetPaymentUrl() != "" || !strings.Contains(booked.GetSpoken(), "Our team will send you the payment details") {
			t.Errorf("%s: %q, url %q, spoken %q; want booked, no link, the team to send the details", what,
				booked.GetReservation(), booked.GetPaymentUrl(), booked.GetSpoken())
		}
	}

	honest("no billing", book(3))

	fin := finance.New(live.FinanceRepositories(conn), res.venues.Venues, nil, nil)
	res.svc.BillWith(fin, 30)
	honest("no gateway", book(4))

	fin.LinkPaymentsWith(&fakeLinker{links: map[string]finance.Link{}})
	box := &inbox{}
	sender := notify.NewSender(box, 0, nil, nil)
	res.svc.SendLinksThrough(sender)
	booked := book(5)
	if !strings.HasPrefix(booked.GetPaymentUrl(), "https://pay.example/") || !strings.Contains(booked.GetSpoken(), "sent a link to pay the advance to your phone") {
		t.Fatalf("with a gateway: url %q, spoken %q; want a link, said sent", booked.GetPaymentUrl(), booked.GetSpoken())
	}
	sender.Wait()
	if len(box.got) != 1 || box.got[0].To != "+9198765"+res.run[len(res.run)-5:] || !strings.Contains(box.got[0].Text, booked.GetPaymentUrl()) ||
		box.got[0].Event != concierge.AdvanceLinkEvent {
		t.Errorf("sent %+v; want the link to the guest's phone", box.got)
	}

	invoices := live.FinanceRepositories(conn).Invoices
	pfs, _, err := invoices.List(ctx, v.GetName(), repox.ListInput{PageSize: 10, Filter: `booking = "` + booked.GetReservation() + `"`})
	if err != nil || len(pfs) != 1 {
		t.Fatalf("invoices for the stay = %d, %v; want one proforma", len(pfs), err)
	}
	pf := pfs[0]
	line := pf.GetLines()[0]
	if pf.GetKind() != financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA || pf.GetState() != financepbv1.Invoice_STATE_ISSUED ||
		line.GetTax().GetHsnSacCode() != "996311" || line.GetTax().GetGstRate().GetValue() != "18" || pf.GetTotal().GetUnits() != 33040 {
		t.Errorf("proforma %v %v, SAC %s at %s%%, total %v; want an issued proforma, 996311 at 18%%, 33,040 rupees",
			pf.GetKind(), pf.GetState(), line.GetTax().GetHsnSacCode(), line.GetTax().GetGstRate().GetValue(), pf.GetTotal())
	}
	pays, _, err := live.FinanceRepositories(conn).Payments.List(ctx, v.GetName(), repox.ListInput{PageSize: 10, Filter: `invoice = "` + pf.GetName() + `"`})
	if err != nil || len(pays) != 1 || pays[0].GetAmount().GetUnits() != 9912 {
		t.Errorf("payments asked = %v, %v; want one of 9,912 rupees, 30%% of 33,040", pays, err)
	}
}

func TestLiveConfirmStayAdvanceRetried(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	res := newResort(t)
	v := res.venue
	v.InvoicePrefix = invoicePrefix()
	if _, err := res.venues.Venues.Update(ctx, v, []string{"invoice_prefix"}); err != nil {
		t.Fatalf("set invoice prefix: %v", err)
	}
	fin := finance.New(live.FinanceRepositories(conn), res.venues.Venues, nil, nil)
	fin.LinkPaymentsWith(&fakeLinker{links: map[string]finance.Link{}})
	res.svc.BillWith(fin, 100)
	held, err := res.svc.HoldStay(ctx, &conciergepbv1.HoldStayRequest{Venue: v.GetName(), Space: res.space.GetName(), Nights: 1, Adults: 2,
		ArrivalDate: &date.Date{Year: int32(time.Now().Year()) + 1, Month: 6, Day: 9}, GuestDisplayName: "John Doe",
		Phone: "98765 " + res.run[len(res.run)-5:], RequestId: uuid.NewString()})
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	req := &conciergepbv1.ConfirmStayRequest{Venue: v.GetName(), HoldToken: held.GetHoldToken(), Confirmed: true}
	first, err1 := res.svc.ConfirmStay(ctx, req)
	again, err2 := res.svc.ConfirmStay(ctx, req)
	if err1 != nil || err2 != nil || first.GetPaymentUrl() == "" || again.GetPaymentUrl() != first.GetPaymentUrl() {
		t.Fatalf("confirm twice = %q (%v), %q (%v); want the same link", first.GetPaymentUrl(), err1, again.GetPaymentUrl(), err2)
	}
	if !strings.Contains(first.GetSpoken(), "You can pay the advance on the payment link") {
		t.Errorf("spoken %q; with no notifier, want the link offered, not said sent", first.GetSpoken())
	}
	invs, _, err := live.FinanceRepositories(conn).Invoices.List(ctx, v.GetName(), repox.ListInput{PageSize: 10, Filter: `booking = "` + first.GetReservation() + `"`})
	if err != nil || len(invs) != 1 {
		t.Errorf("proformas after a retry = %d, %v; want one", len(invs), err)
	}
}
