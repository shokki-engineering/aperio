// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// mcp_headers.go is the MCP middleware MCPService installs (mcp.go): it
// copies each request's Authorization header, which the MCP SDK keeps on
// the request, into the incoming gRPC metadata of the context the tool runs
// in -- where the interceptor reads a gRPC call's.

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/metadata"
)

// liftHeaders installs the middleware on s.
func liftHeaders(s *mcp.Server) {
	s.AddReceivingMiddleware(lift)
}

// lift sets ctx's "authorization" metadata to the request's Authorization
// header, or clears it when the request has none, so only the caller's own
// header ever reaches the interceptor.
func lift(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		md = md.Copy()
		md.Delete("authorization")
		if extra := req.GetExtra(); extra != nil && extra.Header != nil {
			if v := extra.Header.Get("Authorization"); v != "" {
				md.Set("authorization", v)
			}
		}
		return next(metadata.NewIncomingContext(ctx, md), method, req)
	}
}
