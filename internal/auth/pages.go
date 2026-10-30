// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// pages.go is what a venue-limited principal sees of a Global method's
// response (venue.go narrows it): items of other venues are dropped, and
// everything else is kept as the service answered it.
//
// For a List that means a page may hold fewer items than page_size, or
// none, while more follow: the next_page_token is always kept, and the end
// of the list is the page without one -- never merely an empty page
// (AIP-158). So that a client stopping at an empty page is not misled, a
// page that narrowing emptied while a token follows is refilled: the
// interceptor asks for the next page itself, up to maxRefills times, and
// answers the first that keeps any item, with its token. Should every one
// of those be emptied too, the empty page is answered with the token to go
// on from, which is still not the end.

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// maxRefills is how many further pages an emptied page is refilled from.
const maxRefills = 10

// narrowed is resp, the answer to req, as p may see it, refilled from the
// following pages when narrowing emptied it.
func (a *Authenticator) narrowed(ctx context.Context, p Principal, req, resp proto.Message, handler grpc.UnaryHandler) (any, error) {
	for refills := 0; ; refills++ {
		ok, dropped := narrow(p, resp)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "auth: the result belongs to a venue you do not work at")
		}
		token := stringField(resp.ProtoReflect(), "next_page_token")
		if dropped == 0 || items(resp) > 0 || token == "" || refills == maxRefills || req == nil {
			return resp, nil
		}
		next := proto.Clone(req)
		if !setString(next.ProtoReflect(), "page_token", token) {
			return resp, nil
		}
		got, err := handler(ctx, next)
		if err != nil {
			return nil, err
		}
		out, ok := got.(proto.Message)
		if !ok {
			return got, nil
		}
		req, resp = next, out
	}
}

// items is how many items resp's repeated fields of aperio messages hold.
func items(resp proto.Message) int {
	n := 0
	resp.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsList() && fd.Kind() == protoreflect.MessageKind && own(fd.Message()) {
			n += v.List().Len()
		}
		return true
	})
	return n
}

// stringField is m's top-level string field name, "" when it has none.
func stringField(m protoreflect.Message, name protoreflect.Name) string {
	fd := m.Descriptor().Fields().ByName(name)
	if fd == nil || fd.Kind() != protoreflect.StringKind || fd.IsList() {
		return ""
	}
	return m.Get(fd).String()
}

// setString sets m's top-level string field name to v, reporting whether m
// has one.
func setString(m protoreflect.Message, name protoreflect.Name, v string) bool {
	fd := m.Descriptor().Fields().ByName(name)
	if fd == nil || fd.Kind() != protoreflect.StringKind || fd.IsList() {
		return false
	}
	m.Set(fd, protoreflect.ValueOfString(v))
	return true
}
