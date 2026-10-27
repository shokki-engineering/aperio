// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// register.go puts the services on the server's three surfaces -- gRPC, the
// REST gateway and MCP -- through each package's generated RegisterAll.

import (
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/catalog/v1/catalogpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/concierge/v1/conciergepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/document/v1/documentpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/event/v1/eventpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/finance/v1/financepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/guest/v1/guestpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/pricing/v1/pricingpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/stay/v1/staypbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/venue/v1/venuepbv1svc"
	"github.com/the-protobuf-project/runtime-go/grpc"
)

// Options are the hybrid-server options that serve every service on gRPC,
// REST and MCP.
func (s *Services) Options() []grpc.Option {
	var opts []grpc.Option
	for _, o := range [][]grpc.Option{
		venuepbv1svc.RegisterAll(venuepbv1svc.Servers{VenueService: s.Venues}),
		catalogpbv1svc.RegisterAll(catalogpbv1svc.Servers{CatalogService: s.Catalog}),
		pricingpbv1svc.RegisterAll(pricingpbv1svc.Servers{PricingService: s.Pricing}),
		guestpbv1svc.RegisterAll(guestpbv1svc.Servers{GuestService: s.Guests}),
		salespbv1svc.RegisterAll(salespbv1svc.Servers{SalesService: s.Sales}),
		staypbv1svc.RegisterAll(staypbv1svc.Servers{StayService: s.Stays}),
		eventpbv1svc.RegisterAll(eventpbv1svc.Servers{EventService: s.Events}),
		financepbv1svc.RegisterAll(financepbv1svc.Servers{FinanceService: s.Finance}),
		documentpbv1svc.RegisterAll(documentpbv1svc.Servers{DocumentService: s.Documents}),
		operationspbv1svc.RegisterAll(operationspbv1svc.Servers{OperationsService: s.Operations}),
		conciergepbv1svc.RegisterAll(conciergepbv1svc.Servers{ConciergeService: s.Concierge}),
	} {
		opts = append(opts, o...)
	}
	// The webhooks, beside the gateway (routes.go).
	return append(opts, grpc.WithHTTPGateways(s.mountRoutes))
}
