// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// rest_test.go checks the REST surface: as runtime-go serves it, the
// grpc-gateway calls the gRPC server (over an in-memory listener), and the
// HTTP Authorization header reaches the interceptor as the "authorization"
// metadata. A stand-in VenueService answers with the caller's email.

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// venues is a stand-in VenueService: GetVenue answers with the caller's
// email as the venue's display name.
type venues struct {
	venuepbv1.UnimplementedVenueServiceServer
}

// GetVenue is the venue asked for, named after whoever asked.
func (venues) GetVenue(ctx context.Context, req *venuepbv1.GetVenueRequest) (*venuepbv1.Venue, error) {
	p, _ := FromContext(ctx)
	return &venuepbv1.Venue{Name: req.GetName(), DisplayName: p.Email}, nil
}

// TestRESTCarriesTheToken calls GET /v1/venues/lakeside through the
// gateway with and without a token.
func TestRESTCarriesTheToken(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	lis := bufconn.Listen(1 << 20)
	g := grpc.NewServer(grpc.ChainUnaryInterceptor(a.Interceptor()))
	venuepbv1.RegisterVenueServiceServer(g, venues{})
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)

	mux := runtime.NewServeMux()
	dial := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
	}
	if err := venuepbv1.RegisterVenueServiceHandlerFromEndpoint(context.Background(), mux, "passthrough:///bufnet", dial); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	get := func(authorization string) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/venues/lakeside", nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	if code, _ := get(""); code != http.StatusUnauthorized {
		t.Errorf("no token: HTTP %d, want 401", code)
	}
	if code, _ := get("Bearer " + iss.staff(t, []string{"hilltop"}, "front_desk")); code != http.StatusForbidden {
		t.Errorf("another venue's desk: HTTP %d, want 403", code)
	}
	code, body := get("Bearer " + iss.staff(t, []string{"lakeside"}, "front_desk"))
	if code != http.StatusOK || !strings.Contains(body, "john.doe@example.com") {
		t.Errorf("lakeside's desk: HTTP %d %s", code, body)
	}
}
