// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// mcp_server_test.go serves the stand-in VenueService over MCP as
// runtime-go's endpoints do: the generated ServeVenueServiceMCP as a
// combined server, its tools dispatched through the interceptor, with the
// runtime's header mappings (mcp.DefaultHeaderMappings, which the hybrid
// server sets) lifting each request's Authorization into the incoming
// metadata. mcp_test.go is the client.

import (
	"context"
	"net/http"
	"testing"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/the-protobuf-project/runtime-go/agents/mcp"
)

// mcpHandler is the combined MCP server over a's interceptor, as HTTP at
// /mcp; withHeaders false leaves the header mappings out.
func mcpHandler(t *testing.T, a *Authenticator, withHeaders bool) http.Handler {
	t.Helper()
	cfg := &mcp.MCPServerConfig{Name: "aperio", Version: "test", UnaryInterceptor: a.Interceptor()}
	if withHeaders {
		cfg.HeaderMappings = mcp.DefaultHeaderMappings()
	}
	server, err := mcp.CombineServices(context.Background(), cfg, func(ctx context.Context, cfg *mcp.MCPServerConfig) error {
		return venuepbv1.ServeVenueServiceMCP(ctx, venues{}, cfg)
	})
	if err != nil {
		t.Fatal(err)
	}
	h, err := mcp.NewHTTPHandler(server, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
