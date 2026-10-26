// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// service.go is RequestService: what a guest asks for during a stay or an
// event -- two black coffees to room 204, towels, the Wi-Fi -- sent to the
// department that does it as a service request, named after the request id
// so a retry finds it rather than sending a second pot of coffee.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// departments are the departments each kind of ask goes to, and how a guest
// is told who has it.
var departments = map[conciergepbv1.ServiceKind]struct {
	dept operationspbv1.ServiceRequest_Department
	who  string
}{
	conciergepbv1.ServiceKind_SERVICE_KIND_DRINKS:       {operationspbv1.ServiceRequest_DEPARTMENT_PANTRY, "the pantry"},
	conciergepbv1.ServiceKind_SERVICE_KIND_FOOD:         {operationspbv1.ServiceRequest_DEPARTMENT_KITCHEN, "the kitchen"},
	conciergepbv1.ServiceKind_SERVICE_KIND_HOUSEKEEPING: {operationspbv1.ServiceRequest_DEPARTMENT_HOUSEKEEPING, "housekeeping"},
	conciergepbv1.ServiceKind_SERVICE_KIND_REPAIR:       {operationspbv1.ServiceRequest_DEPARTMENT_MAINTENANCE, "maintenance"},
	conciergepbv1.ServiceKind_SERVICE_KIND_WIFI_AV:      {operationspbv1.ServiceRequest_DEPARTMENT_IT_AV, "the IT team"},
	conciergepbv1.ServiceKind_SERVICE_KIND_FRONT_DESK:   {operationspbv1.ServiceRequest_DEPARTMENT_FRONT_DESK, "the front desk"},
}

// RequestService sends a guest's ask to the department that does it.
func (s *Server) RequestService(ctx context.Context, req *conciergepbv1.RequestServiceRequest) (*conciergepbv1.RequestServiceResponse, error) {
	return run(ctx, s, "RequestService", func(ctx context.Context) (*conciergepbv1.RequestServiceResponse, error) {
		d, ok := departments[req.GetKind()]
		if !ok {
			d = departments[conciergepbv1.ServiceKind_SERVICE_KIND_FRONT_DESK]
		}
		qty, what, where := max(req.GetQuantity(), 1), strings.TrimSpace(req.GetWhat()), strings.TrimSpace(req.GetLocation())
		priority := operationspbv1.ServiceRequest_PRIORITY_NORMAL
		if req.GetUrgent() {
			priority = operationspbv1.ServiceRequest_PRIORITY_URGENT
		}
		id := "svc-" + strings.ToLower(req.GetRequestId())
		sr, err := s.ops.CreateServiceRequest(ctx, &operationspbv1.CreateServiceRequestRequest{
			Parent: req.GetVenue(), ServiceRequestId: id, ServiceRequest: &operationspbv1.ServiceRequest{
				Summary: fmt.Sprintf("%d × %s to %s", qty, what, where), Department: d.dept, Location: where,
				Items:    []*operationspbv1.RequestItem{{DisplayName: what, Quantity: qty, Notes: req.GetNotes()}},
				Priority: priority, Notes: req.GetNotes(),
			},
		})
		if status.Code(err) == codes.AlreadyExists {
			sr, err = s.ops.GetServiceRequest(ctx, &operationspbv1.GetServiceRequestRequest{Name: req.GetVenue() + "/serviceRequests/" + id})
		}
		if err != nil {
			return nil, err
		}
		return &conciergepbv1.RequestServiceResponse{
			ServiceRequest: sr.GetName(),
			Spoken:         fmt.Sprintf("Done: %d %s to %s. %s has it.", qty, what, where, capitalize(d.who)),
		}, nil
	})
}

// capitalize is s with its first letter upper-cased, to start a sentence.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
