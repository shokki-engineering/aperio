// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// stream_test.go checks the stream interceptor: health open, reflection for
// signed-in staff only, and nothing guarded while auth is off.

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// reflectionInfo is gRPC server reflection's one method.
const reflectionInfo = "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo"

// fakeStream is a server stream with only a context.
type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s fakeStream) Context() context.Context { return s.ctx }

// openStream runs the interceptor for method with authorization (none
// when empty), reporting the code and the caller the handler saw.
func openStream(a *Authenticator, method, authorization string) (codes.Code, Principal) {
	ctx := context.Background()
	if authorization != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authorization))
	}
	var seen Principal
	err := a.StreamInterceptor()(nil, fakeStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: method},
		func(_ any, ss grpc.ServerStream) error {
			seen, _ = FromContext(ss.Context())
			return nil
		})
	return status.Code(err), seen
}

func TestStreamInterceptor(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	staff := "Bearer " + iss.staff(t, []string{"lakeside"}, "kitchen")
	token, err := a.Links().SignLink("venues/lakeside/invoices/i1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, method, authorization string
		want                        codes.Code
	}{
		{"health, anonymous", "/grpc.health.v1.Health/Watch", "", codes.OK},
		{"reflection, anonymous", reflectionInfo, "", codes.Unauthenticated},
		{"reflection, a bad token", reflectionInfo, "Bearer nope", codes.Unauthenticated},
		{"reflection, a guest's link", reflectionInfo, "Link " + token, codes.PermissionDenied},
		{"reflection, staff", reflectionInfo, staff, codes.OK},
	}
	for _, c := range cases {
		if got, _ := openStream(a, c.method, c.authorization); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if _, p := openStream(a, reflectionInfo, staff); !p.Has(Kitchen) {
		t.Errorf("the handler saw %+v, want the kitchen's principal", p)
	}
}

// TestStreamInterceptorOff lets every stream through while auth is off.
func TestStreamInterceptorOff(t *testing.T) {
	cfg := newIssuer(t).config()
	cfg.Issuer = ""
	a, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := openStream(a, reflectionInfo, ""); got != codes.OK {
		t.Errorf("auth off: %v, want OK", got)
	}
}
