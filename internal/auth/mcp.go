// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// mcp.go carries a caller's token across MCP. The generated tool handlers
// dispatch through the server's unary interceptor chain (mcp.InvokeUnary),
// but with the context the MCP SDK gives them, which holds no HTTP header:
// the SDK keeps each request's headers on the request (RequestExtra), and
// the runtime forwards none to the chain. MCPService is an MCP service with
// no tools of its own: registered beside the generated ones, it is combined
// into the one server at the MCP endpoint and installs a middleware there
// (mcp_headers.go) that lifts each call's Authorization header into the
// incoming gRPC metadata the interceptor reads.
//
// It covers the combined endpoint only. The per-service endpoints are
// separate servers it cannot reach; calls to them carry no token and are
// refused (public ones aside), so serve the combined endpoint alone
// (grpc.WithoutMCPServicePaths).

import (
	"context"

	"github.com/the-protobuf-project/runtime-go/agents/mcp"
)

// mcpBasePath is where the service would be served on its own, when the
// runtime serves each service at its own path too: an empty server, apart
// from every other path.
const mcpBasePath = "/aperio/auth/v1/mcp"

// MCPService is the MCP service to register with grpc.WithMCPServices
// beside the generated ones (it is a grpc.MCPServiceFunc).
func MCPService() func(context.Context, *mcp.MCPServerConfig) error {
	return func(ctx context.Context, cfg *mcp.MCPServerConfig) error {
		cfg.GeneratedBasePath = mcpBasePath
		return mcp.StartServer(ctx, cfg, liftHeaders)
	}
}
