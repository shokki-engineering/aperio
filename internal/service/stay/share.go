// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// share.go is ShareReservation: a link the guest opens to read one
// reservation without signing in -- the guest site's page for it with a
// token signed by auth.Links, read-only, for that reservation alone,
// lapsing after the request's ttl (14 days unset). GetReservation takes the
// token (auth's link policy).
//
// ShareLinksWith sets what signs the links; without it ShareReservation is
// Unimplemented.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/auth"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// sharer signs links to base, the guest site.
type sharer struct {
	links *auth.Links
	base  string
}

// ShareLinksWith signs ShareReservation's links with links, to pages under base,
// the guest site's URL.
func (s *Server) ShareLinksWith(links *auth.Links, base string) {
	s.share = sharer{links: links, base: base}
}

// ShareReservation signs a read-only link to one reservation for the guest.
func (s *Server) ShareReservation(ctx context.Context, req *staypbv1.ShareReservationRequest) (*staypbv1.ShareReservationResponse, error) {
	var out *staypbv1.ShareReservationResponse
	err := s.Options().Run(ctx, "StayService", "ShareReservation", func(ctx context.Context) error {
		sh := s.share
		if sh.links == nil {
			return status.Error(codes.Unimplemented, "ShareReservation: no link signer is configured")
		}
		r, err := s.Repositories().Reservations.Get(ctx, req.GetName())
		if err != nil {
			return servicex.NotFoundAs(err, reservationType, req.GetName())
		}
		url, expire, err := sh.links.ShareLink(sh.base, r.GetName(), req.GetTtl().AsDuration())
		if err != nil {
			return err
		}
		out = &staypbv1.ShareReservationResponse{Url: url, ExpireTime: timestamppb.New(expire)}
		return nil
	})
	return out, err
}
