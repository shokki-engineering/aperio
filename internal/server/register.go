// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// register.go puts the services on the server's three surfaces -- gRPC, the
// REST gateway and MCP -- through each package's generated RegisterAll.

import (
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/booking/v1/bookingpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/concierge/v1/conciergepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/crm/v1/crmpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/document/v1/documentpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/finance/v1/financepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/venue/v1/venuepbv1svc"
	"github.com/the-protobuf-project/runtime-go/grpc"
)

// Options are the hybrid-server options that serve every service on gRPC,
// REST and MCP.
func (s *Services) Options() []grpc.Option {
	var opts []grpc.Option
	for _, o := range [][]grpc.Option{
		venuepbv1svc.RegisterAll(venuepbv1svc.Servers{VenueService: s.Venues}),
		crmpbv1svc.RegisterAll(crmpbv1svc.Servers{CrmService: s.CRM}),
		bookingpbv1svc.RegisterAll(bookingpbv1svc.Servers{BookingService: s.Bookings}),
		salespbv1svc.RegisterAll(salespbv1svc.Servers{SalesService: s.Sales}),
		financepbv1svc.RegisterAll(financepbv1svc.Servers{FinanceService: s.Finance}),
		documentpbv1svc.RegisterAll(documentpbv1svc.Servers{DocumentService: s.Documents}),
		operationspbv1svc.RegisterAll(operationspbv1svc.Servers{OperationsService: s.Operations}),
		conciergepbv1svc.RegisterAll(conciergepbv1svc.Servers{ConciergeService: s.Concierge}),
	} {
		opts = append(opts, o...)
	}
	return opts
}
