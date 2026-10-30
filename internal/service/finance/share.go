// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// share.go is ShareInvoice: a link the guest opens to read one invoice
// without signing in -- the guest site's page for it with a token signed by
// auth.Links, read-only, for that invoice alone, lapsing after the
// request's ttl (14 days unset). GetInvoice takes the token (auth's link
// policy). Without ShareLinksWith, ShareInvoice is Unimplemented.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/auth"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ShareLinksWith signs ShareInvoice's links with links, to pages under base,
// the guest site's URL.
func (s *Server) ShareLinksWith(links *auth.Links, base string) {
	s.h.links, s.h.linkBase = links, base
}

// ShareInvoice signs a read-only link to one invoice for the guest.
func (s *Server) ShareInvoice(ctx context.Context, req *financepbv1.ShareInvoiceRequest) (*financepbv1.ShareInvoiceResponse, error) {
	var out *financepbv1.ShareInvoiceResponse
	err := s.Options().Run(ctx, "FinanceService", "ShareInvoice", func(ctx context.Context) error {
		if s.h.links == nil {
			return status.Error(codes.Unimplemented, "ShareInvoice: no link signer is configured")
		}
		inv, err := s.Repositories().Invoices.Get(ctx, req.GetName())
		if err != nil {
			return servicex.NotFoundAs(err, invoiceType, req.GetName())
		}
		url, expire, err := s.h.links.ShareLink(s.h.linkBase, inv.GetName(), req.GetTtl().AsDuration())
		if err != nil {
			return err
		}
		out = &financepbv1.ShareInvoiceResponse{Url: url, ExpireTime: timestamppb.New(expire)}
		return nil
	})
	return out, err
}
