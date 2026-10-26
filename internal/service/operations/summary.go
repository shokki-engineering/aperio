// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations

// summary.go is SummarizeServiceRequests: a venue's requests as the duty
// manager reads them -- each department's open queue, how much of it is
// late, and how many requests were done today and how fast. It costs two
// lists, whatever the venue's size: its open requests, oldest first, and
// those completed today; nothing is read per request.

import (
	"context"
	"slices"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/gorm/filterx"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
)

// briefsPerQueue is how many requests a DepartmentQueue lists.
const briefsPerQueue = 20

// openStates are the states of a request not yet completed or cancelled, as
// a filter names them.
var openStates = []string{"OPEN", "ACKNOWLEDGED", "IN_PROGRESS"}

// briefMask is what a RequestBrief reads of a request.
var briefMask = []string{"summary", "location", "state", "priority", "due_time", "create_time", "assignee", "department"}

// SummarizeServiceRequests summarizes req.Venue's requests, of
// req.Department only when it is set.
func (s *Server) SummarizeServiceRequests(ctx context.Context, req *operationspbv1.SummarizeServiceRequestsRequest) (*operationspbv1.SummarizeServiceRequestsResponse, error) {
	var out *operationspbv1.SummarizeServiceRequestsResponse
	err := s.Options().Run(ctx, "OperationsService", "SummarizeServiceRequests", func(ctx context.Context) error {
		var err error
		out, err = s.h.summarize(ctx, req)
		return err
	})
	return out, err
}

func (h *hooks) summarize(ctx context.Context, req *operationspbv1.SummarizeServiceRequestsRequest) (*operationspbv1.SummarizeServiceRequestsResponse, error) {
	zone, err := h.zone(ctx, req.GetVenue())
	if err != nil {
		return nil, err
	}
	now := h.now()
	var where []filterx.Condition
	if d := req.GetDepartment(); d != operationspbv1.ServiceRequest_DEPARTMENT_UNSPECIFIED {
		where = append(where, filterx.In("department", d.String()))
	}
	open, err := h.all(ctx, req.GetVenue(), repox.ListInput{OrderBy: "create_time", ReadMask: briefMask,
		Where: append(slices.Clone(where), filterx.In("state", openStates...))})
	if err != nil {
		return nil, err
	}
	local := now.In(zone)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
	done, err := h.all(ctx, req.GetVenue(), repox.ListInput{ReadMask: []string{"create_time", "complete_time"},
		Where: append(where, filterx.In("state", "DONE"),
			filterx.Condition{Field: "complete_time", Op: filterx.OpGte, Value: midnight.Format(time.RFC3339)},
			filterx.Condition{Field: "complete_time", Op: filterx.OpLt, Value: midnight.AddDate(0, 0, 1).Format(time.RFC3339)})})
	if err != nil {
		return nil, err
	}
	return &operationspbv1.SummarizeServiceRequestsResponse{
		Departments: queues(open, now), CompletedToday: int32(len(done)), MedianMinutes: medianMinutes(done),
	}, nil
}

// all lists every request of venueName in matches, page by page.
func (h *hooks) all(ctx context.Context, venueName string, in repox.ListInput) ([]*operationspbv1.ServiceRequest, error) {
	in.PageSize = 1000
	var out []*operationspbv1.ServiceRequest
	for {
		page, next, err := h.repos.ServiceRequests.List(ctx, venueName, in)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			return out, nil
		}
		in.PageToken = next
	}
}

// queues are the departments of open, oldest first, as of now: the most
// overdue first, then the most open, then in the enum's order.
func queues(open []*operationspbv1.ServiceRequest, now time.Time) []*operationspbv1.DepartmentQueue {
	byDept := map[operationspbv1.ServiceRequest_Department]*operationspbv1.DepartmentQueue{}
	var out []*operationspbv1.DepartmentQueue
	for _, r := range open {
		q := byDept[r.GetDepartment()]
		if q == nil {
			q = &operationspbv1.DepartmentQueue{Department: r.GetDepartment()}
			byDept[r.GetDepartment()] = q
			out = append(out, q)
		}
		q.Open++
		if r.GetDueTime() != nil && r.GetDueTime().AsTime().Before(now) {
			q.Overdue++
		}
		if len(q.Requests) < briefsPerQueue {
			q.Requests = append(q.Requests, brief(r))
		}
	}
	slices.SortFunc(out, func(a, b *operationspbv1.DepartmentQueue) int {
		switch {
		case a.Overdue != b.Overdue:
			return int(b.Overdue - a.Overdue)
		case a.Open != b.Open:
			return int(b.Open - a.Open)
		}
		return int(a.Department - b.Department)
	})
	return out
}

// brief is r as its department reads it.
func brief(r *operationspbv1.ServiceRequest) *operationspbv1.RequestBrief {
	return &operationspbv1.RequestBrief{
		ServiceRequest: r.GetName(), Summary: r.GetSummary(), Location: r.GetLocation(), State: r.GetState(),
		Priority: r.GetPriority(), DueTime: r.GetDueTime(), CreateTime: r.GetCreateTime(), Assignee: r.GetAssignee(),
	}
}

// medianMinutes is the median of done's minutes from asked to done; 0 when
// there are none.
func medianMinutes(done []*operationspbv1.ServiceRequest) int32 {
	if len(done) == 0 {
		return 0
	}
	took := make([]time.Duration, 0, len(done))
	for _, r := range done {
		took = append(took, r.GetCompleteTime().AsTime().Sub(r.GetCreateTime().AsTime()))
	}
	slices.Sort(took)
	mid := took[len(took)/2]
	if len(took)%2 == 0 {
		mid = (took[len(took)/2-1] + mid) / 2
	}
	return int32(mid.Minutes())
}
