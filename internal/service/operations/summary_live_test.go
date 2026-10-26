// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations_test

// A live test of SummarizeServiceRequests over the dev database: queues of
// four departments ordered most overdue, then most open; each queue's
// requests oldest first; a completed and a cancelled request left out of the
// open ones; today's completed requests and their median minutes; one
// department only; and another venue's request left out.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/service/operations/ -v

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/operations"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Departments, by their short Go names.
const (
	pantry      = operationspbv1.ServiceRequest_DEPARTMENT_PANTRY
	kitchen     = operationspbv1.ServiceRequest_DEPARTMENT_KITCHEN
	housekeep   = operationspbv1.ServiceRequest_DEPARTMENT_HOUSEKEEPING
	maintenance = operationspbv1.ServiceRequest_DEPARTMENT_MAINTENANCE
)

func TestLiveSummarizeServiceRequests(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	// A zone in which the next hour is still today, so requests made and
	// done within it count as today's.
	zone := "UTC"
	if h := time.Now().UTC().Hour(); h == 0 || h == 23 {
		zone = "Asia/Kolkata"
	}
	venues := venue.New(conn)
	repos := live.OperationsRepositories(conn)
	svc := operations.New(repos, venues.Venues)
	newVenue := func(name string) string {
		v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: name + " " + fmt.Sprint(time.Now().UnixNano()),
			Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS, TimeZone: zone, CurrencyCode: "INR",
			Address: &postaladdress.PostalAddress{RegionCode: "IN"}, Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
		must(err, "create venue")
		return v.GetName()
	}
	here, there := newVenue("Ops Resort"), newVenue("Other Resort")

	past, future := timestamppb.New(time.Now().Add(-time.Hour)), timestamppb.New(time.Now().Add(time.Hour))
	ask := func(venueName string, d operationspbv1.ServiceRequest_Department, summary string, due *timestamppb.Timestamp) *operationspbv1.ServiceRequest {
		t.Helper()
		r, err := svc.CreateServiceRequest(ctx, &operationspbv1.CreateServiceRequestRequest{Parent: venueName, ServiceRequest: &operationspbv1.ServiceRequest{
			Summary: summary, Department: d, Location: "204", DueTime: due, Requester: "John Doe"}})
		must(err, "create "+summary)
		return r
	}
	// end moves r to state; a done one done after minutes.
	end := func(r *operationspbv1.ServiceRequest, state operationspbv1.ServiceRequest_State, minutes int) {
		t.Helper()
		r.State = state
		paths := []string{"state"}
		if state == operationspbv1.ServiceRequest_STATE_DONE {
			r.CompleteTime, paths = timestamppb.New(r.GetCreateTime().AsTime().Add(time.Duration(minutes)*time.Minute)), append(paths, "complete_time")
		}
		_, err := repos.ServiceRequests.UpdateOutputOnly(ctx, r, paths)
		must(err, "end "+r.GetSummary())
	}

	p1, p2, p3 := ask(here, pantry, "2 black coffee", past), ask(here, pantry, "Water", nil), ask(here, pantry, "Tea", future)
	end(ask(here, pantry, "Snacks", nil), operationspbv1.ServiceRequest_STATE_DONE, 10)
	end(ask(here, pantry, "Juice", nil), operationspbv1.ServiceRequest_STATE_CANCELLED, 0)
	ask(here, housekeep, "Towels", past)
	ask(here, housekeep, "Linen", past)
	end(ask(here, housekeep, "Clean room", nil), operationspbv1.ServiceRequest_STATE_DONE, 30)
	ask(here, kitchen, "Dinner", nil)
	ask(here, kitchen, "Lunch", future)
	end(ask(here, kitchen, "Breakfast", nil), operationspbv1.ServiceRequest_STATE_DONE, 20)
	ask(here, maintenance, "Fix the AC", nil)
	ask(there, pantry, "Elsewhere", past)

	sum := func(d operationspbv1.ServiceRequest_Department) *operationspbv1.SummarizeServiceRequestsResponse {
		t.Helper()
		out, err := svc.SummarizeServiceRequests(ctx, &operationspbv1.SummarizeServiceRequestsRequest{Venue: here, Department: d})
		must(err, "summarize")
		return out
	}
	queue := func(q *operationspbv1.DepartmentQueue) string {
		return fmt.Sprintf("%s open=%d overdue=%d briefs=%d", q.GetDepartment(), q.GetOpen(), q.GetOverdue(), len(q.GetRequests()))
	}

	all := sum(operationspbv1.ServiceRequest_DEPARTMENT_UNSPECIFIED)
	want := []string{
		"DEPARTMENT_HOUSEKEEPING open=2 overdue=2 briefs=2", "DEPARTMENT_PANTRY open=3 overdue=1 briefs=3",
		"DEPARTMENT_KITCHEN open=2 overdue=0 briefs=2", "DEPARTMENT_MAINTENANCE open=1 overdue=0 briefs=1",
	}
	if len(all.GetDepartments()) != len(want) {
		t.Fatalf("departments = %d, want %d: %v", len(all.GetDepartments()), len(want), all.GetDepartments())
	}
	for i, q := range all.GetDepartments() {
		if got := queue(q); got != want[i] {
			t.Errorf("queue %d = %q, want %q", i, got, want[i])
		}
	}
	if all.GetCompletedToday() != 3 || all.GetMedianMinutes() != 20 {
		t.Errorf("completed today = %d, median = %d; want 3, 20", all.GetCompletedToday(), all.GetMedianMinutes())
	}

	// Pantry only: oldest first, its own completed request.
	one := sum(pantry)
	if len(one.GetDepartments()) != 1 || queue(one.GetDepartments()[0]) != want[1] {
		t.Fatalf("pantry = %v", one.GetDepartments())
	}
	for i, r := range []*operationspbv1.ServiceRequest{p1, p2, p3} {
		b := one.GetDepartments()[0].GetRequests()[i]
		if b.GetServiceRequest() != r.GetName() || b.GetSummary() != r.GetSummary() || b.GetLocation() != "204" ||
			b.GetState() != operationspbv1.ServiceRequest_STATE_OPEN {
			t.Errorf("pantry brief %d = %v, want %s", i, b, r.GetName())
		}
	}
	if one.GetCompletedToday() != 1 || one.GetMedianMinutes() != 10 {
		t.Errorf("pantry completed today = %d, median = %d; want 1, 10", one.GetCompletedToday(), one.GetMedianMinutes())
	}

	// Nothing open in the front desk: no queue, nothing completed.
	if none := sum(operationspbv1.ServiceRequest_DEPARTMENT_FRONT_DESK); len(none.GetDepartments()) != 0 || none.GetCompletedToday() != 0 || none.GetMedianMinutes() != 0 {
		t.Errorf("front desk = %v", none)
	}
}
