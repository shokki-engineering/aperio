// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations

// notify.go tells people about requests, through the notify.Sender the
// server is given: the request's department the moment it is made, at the
// destination the configuration maps it to ([notify.departments]); and,
// when it is done, the guest -- the requester when it is a contact, else
// the contact of its booking -- on their first phone, when there is one.
// A message that cannot be sent is the Sender's to log and count; the
// request stands either way.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/internal/notify"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
)

// CompletedEvent is the event a guest is told a request is done under.
const CompletedEvent = "aperio.operations.service_request.completed"

// fallbackRoute is the [notify.departments] key a department not listed is
// told through.
const fallbackRoute = "default"

// routes are where each department is told.
type routes map[operationspbv1.ServiceRequest_Department]string

// DepartmentKey is the configuration's name for d: "pantry", "it_av".
func DepartmentKey(d operationspbv1.ServiceRequest_Department) string {
	return strings.ToLower(strings.TrimPrefix(d.String(), "DEPARTMENT_"))
}

// NotifyThrough has s tell departments and guests through sender;
// departments maps a DepartmentKey, or "default", to its destination. It
// fails on a key that names no department.
func (s *Server) NotifyThrough(sender *notify.Sender, departments map[string]string) error {
	byKey := map[string]operationspbv1.ServiceRequest_Department{}
	for v := range operationspbv1.ServiceRequest_Department_name {
		if d := operationspbv1.ServiceRequest_Department(v); d != operationspbv1.ServiceRequest_DEPARTMENT_UNSPECIFIED {
			byKey[DepartmentKey(d)] = d
		}
	}
	r := routes{}
	for k, to := range departments {
		d, ok := byKey[k]
		if !ok && k != fallbackRoute {
			return fmt.Errorf("operations: notify.departments.%s names no department", k)
		}
		r[d] = to // the fallback is kept under DEPARTMENT_UNSPECIFIED
	}
	s.h.sender, s.h.routes = sender, r
	return nil
}

// AfterCreateServiceRequest tells the request's department.
func (h *hooks) AfterCreateServiceRequest(ctx context.Context, m *operationspbv1.ServiceRequest) error {
	to, ok := h.routes[m.GetDepartment()]
	if !ok {
		to = h.routes[operationspbv1.ServiceRequest_DEPARTMENT_UNSPECIFIED]
	}
	h.sender.Send(ctx, notify.Message{To: to, Event: operationspbv1svc.ServiceRequestCreatedEvent, Subject: m.GetName(), Text: askText(m)})
	return nil
}

// tellRequester tells the guest r is for that it is done.
func (h *hooks) tellRequester(ctx context.Context, r *operationspbv1.ServiceRequest) {
	if h.sender == nil {
		return
	}
	contact := r.GetRequester()
	if !strings.HasPrefix(contact, "contacts/") {
		contact = ""
		if b := billed(r.GetBooking()); b != "" {
			contact, _ = h.contactOf(ctx, b)
		}
	}
	if phone := h.phoneOf(ctx, contact); phone != "" {
		h.sender.Send(ctx, notify.Message{To: phone, Event: CompletedEvent, Subject: r.GetName(), Text: "Done: " + r.GetSummary() + "."})
	}
}

// askText is r as its department reads it: "URGENT: 2 × black coffee to 204
// -- 2 × Black coffee (no sugar). Use the back stairs."
func askText(r *operationspbv1.ServiceRequest) string {
	var b strings.Builder
	if r.GetPriority() == operationspbv1.ServiceRequest_PRIORITY_URGENT {
		b.WriteString("URGENT: ")
	}
	b.WriteString(r.GetSummary())
	if loc := r.GetLocation(); loc != "" && !strings.Contains(r.GetSummary(), loc) {
		b.WriteString(" at " + loc)
	}
	var items []string
	for _, it := range r.GetItems() {
		item := fmt.Sprintf("%d × %s", it.GetQuantity(), it.GetDisplayName())
		if it.GetNotes() != "" {
			item += " (" + it.GetNotes() + ")"
		}
		items = append(items, item)
	}
	if len(items) > 0 {
		b.WriteString(" -- " + strings.Join(items, ", ") + ".")
	}
	if n := r.GetNotes(); n != "" && (len(r.GetItems()) != 1 || r.GetItems()[0].GetNotes() != n) {
		b.WriteString(" " + n)
	}
	return b.String()
}
