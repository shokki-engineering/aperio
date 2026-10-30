// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// share.go is ShareQuote: a link the guest opens to read one quote without
// signing in -- the guest site's page for it with a token signed by
// auth.Links, read-only, for that quote alone, lapsing after the request's
// ttl (14 days unset). GetQuote takes the token (auth's link policy).
//
// ShareLinksWith sets what signs the links; without it ShareQuote is
// Unimplemented.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/auth"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// sharer signs links to base, the guest site.
type sharer struct {
	links *auth.Links
	base  string
}

// ShareLinksWith signs ShareQuote's links with links, to pages under base,
// the guest site's URL.
func (s *Server) ShareLinksWith(links *auth.Links, base string) {
	s.share = sharer{links: links, base: base}
}

// ShareQuote signs a read-only link to one quote for the guest.
func (s *Server) ShareQuote(ctx context.Context, req *salespbv1.ShareQuoteRequest) (*salespbv1.ShareQuoteResponse, error) {
	var out *salespbv1.ShareQuoteResponse
	err := s.Options().Run(ctx, "SalesService", "ShareQuote", func(ctx context.Context) error {
		sh := s.share
		if sh.links == nil {
			return status.Error(codes.Unimplemented, "ShareQuote: no link signer is configured")
		}
		q, err := s.Repositories().Quotes.Get(ctx, req.GetName())
		if err != nil {
			return servicex.NotFoundAs(err, quoteType, req.GetName())
		}
		url, expire, err := sh.links.ShareLink(sh.base, q.GetName(), req.GetTtl().AsDuration())
		if err != nil {
			return err
		}
		out = &salespbv1.ShareQuoteResponse{Url: url, ExpireTime: timestamppb.New(expire)}
		return nil
	})
	return out, err
}
