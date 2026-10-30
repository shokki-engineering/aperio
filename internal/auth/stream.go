// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// stream.go guards the streams the server serves. aperio's own services
// have no streaming methods; the streams are the runtime's -- gRPC server
// reflection, which describes every service and message, and gRPC health's
// Watch. Health stays open, for probes; reflection, and any other stream
// outside aperio's services, takes a signed-in member of staff (any role;
// a guest's link reads one resource, not the schema). A streaming aperio
// method, should one come, is held to its policy as a unary call is, with
// no request to read venues from.

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// healthService is gRPC health's service, open to every caller.
const healthService = "/grpc.health.v1.Health/"

// StreamInterceptor is the stream interceptor that authenticates every
// stream but health's; like Interceptor, it lets everything through when a
// is off.
func (a *Authenticator) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if a.verifier == nil || strings.HasPrefix(info.FullMethod, healthService) {
			return handler(srv, ss)
		}
		ctx := ss.Context()
		rule, known := a.policy.Rule(info.FullMethod)
		if Guarded(info.FullMethod) && known && rule.Public {
			return handler(srv, ss)
		}
		p, err := a.principal(ctx)
		if err != nil {
			return err
		}
		if Guarded(info.FullMethod) {
			if err := a.authorize(ctx, p, rule, known, nil); err != nil {
				return err
			}
		} else if p.Link != "" {
			return status.Error(codes.PermissionDenied, "auth: a link reads only the one resource it was sent for")
		}
		return handler(srv, principalStream{ServerStream: ss, p: p})
	}
}

// principalStream is a stream whose context carries its caller.
type principalStream struct {
	grpc.ServerStream
	p Principal
}

// Context is the stream's context with its principal (WithPrincipal).
func (s principalStream) Context() context.Context {
	return WithPrincipal(s.ServerStream.Context(), s.p)
}
