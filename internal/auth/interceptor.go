// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// interceptor.go applies the policy to every call. It reads the
// "authorization" metadata -- gRPC's own, the REST gateway's (grpc-gateway
// forwards the HTTP Authorization header under that key) or MCP's (mcp.go
// lifts it there) -- as "Bearer <OIDC token>" for staff or "Link <token>"
// for a guest's link, and lets the call through only when the policy does.

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Interceptor is the unary interceptor that authenticates and authorizes
// every aperio call. Methods outside aperio's services (gRPC health) are let
// through, as is everything when a is off.
func (a *Authenticator) Interceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if a.verifier == nil || !Guarded(info.FullMethod) {
			return handler(ctx, req)
		}
		rule, known := a.policy.Rule(info.FullMethod)
		if known && rule.Public {
			return handler(ctx, req)
		}
		p, err := a.principal(ctx)
		if err != nil {
			return nil, err
		}
		msg, _ := req.(proto.Message)
		if err := a.authorize(ctx, p, rule, known, msg); err != nil {
			return nil, err
		}
		ctx = WithPrincipal(ctx, p)
		resp, err := handler(ctx, req)
		if err != nil || !rule.Global || p.Unlimited() {
			return resp, err
		}
		if out, ok := resp.(proto.Message); ok {
			return a.narrowed(ctx, p, msg, out, handler)
		}
		return resp, nil
	}
}

// authorize decides whether p may make a call under rule (known false when
// the policy has none) with request msg.
func (a *Authenticator) authorize(ctx context.Context, p Principal, rule Rule, known bool, msg proto.Message) error {
	if p.Link != "" {
		if !known || !rule.Link || msg == nil || requestName(msg) != p.Link {
			return status.Error(codes.PermissionDenied, "auth: a link reads only the one resource it was sent for")
		}
		return nil
	}
	if !known {
		if p.Has(Owner) && p.Unlimited() {
			return nil
		}
		return status.Error(codes.PermissionDenied, "auth: this method has no access policy yet")
	}
	if !rule.Allows(p) {
		return status.Errorf(codes.PermissionDenied, "auth: your roles (%s) may not do this", roleList(p))
	}
	if p.Unlimited() {
		return nil
	}
	if rule.Unlimited {
		return status.Error(codes.PermissionDenied, "auth: only someone working at every venue may do this")
	}
	var venues []string
	if msg != nil {
		venues = Venues(msg)
	}
	leadVenues, err := a.leadVenues(ctx, msg)
	if err != nil {
		return err
	}
	venues = append(venues, leadVenues...)
	if v, ok := reachesAll(p, venues); !ok {
		if v == "venues/-" {
			return status.Error(codes.PermissionDenied, "auth: you work at some venues only; ask venue by venue, not venues/-")
		}
		return status.Errorf(codes.PermissionDenied, "auth: you do not work at %s", v)
	}
	if len(venues) == 0 && !rule.Global {
		return status.Error(codes.PermissionDenied, "auth: the request names no venue, and you work at some venues only")
	}
	return nil
}

// principal is who the call's authorization metadata says is calling.
func (a *Authenticator) principal(ctx context.Context) (Principal, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("authorization")
	if len(values) == 0 {
		return Principal{}, status.Error(codes.Unauthenticated, "auth: sign in: send Authorization: Bearer <token>")
	}
	scheme, token, _ := strings.Cut(strings.TrimSpace(values[0]), " ")
	token = strings.TrimSpace(token)
	switch strings.ToLower(scheme) {
	case "bearer":
		p, err := a.verifier.Verify(ctx, token)
		if err != nil {
			return Principal{}, status.Error(codes.Unauthenticated, "auth: the token is not valid: "+err.Error())
		}
		return p, nil
	case "link":
		resource, err := a.links.VerifyLink(token)
		if err != nil {
			return Principal{}, status.Error(codes.Unauthenticated, err.Error())
		}
		return Principal{Subject: "link", Link: resource}, nil
	}
	return Principal{}, status.Error(codes.Unauthenticated, "auth: Authorization must be Bearer <token> or Link <token>")
}

// requestName is a request's top-level name field, as a Get carries it.
func requestName(m proto.Message) string {
	r := m.ProtoReflect()
	fd := r.Descriptor().Fields().ByName("name")
	if fd == nil || fd.IsList() || fd.Message() != nil {
		return ""
	}
	return r.Get(fd).String()
}

// roleList is p's roles, comma-separated, or "none".
func roleList(p Principal) string {
	if len(p.Roles) == 0 {
		return "none"
	}
	names := make([]string, len(p.Roles))
	for i, r := range p.Roles {
		names[i] = string(r)
	}
	return strings.Join(names, ", ")
}
