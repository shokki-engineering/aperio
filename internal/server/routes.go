// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// routes.go is the plain HTTP endpoints aperio serves beside its REST
// gateway on the REST port: the webhooks payment gateways, freebusy and the
// channels deliver to. They are added to the gateway's own mux
// (runtime.ServeMux.HandlePath) rather than replacing it -- runtime-go's
// WithHTTPHandler would take the gateway's place -- so /v1/... and /health
// keep being served as they are. register.go mounts them; this file imports
// google.golang.org/grpc, whose package name runtime-go's shares.

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// route is one plain HTTP endpoint.
type route struct {
	method, path string
	handler      http.Handler
}

// Route serves h for method and path on the REST port, beside the gateway.
// Routes are mounted when the server is built (Options), so add them first.
func (s *Services) Route(method, path string, h http.Handler) {
	s.routes = append(s.routes, route{method: method, path: path, handler: h})
}

// mountRoutes adds every route to the gateway's mux: a runtime-go
// HTTPServiceFunc.
func (s *Services) mountRoutes(mux *runtime.ServeMux, _ string, _ []grpc.DialOption) error {
	for _, r := range s.routes {
		h := r.handler
		if err := mux.HandlePath(r.method, r.path, func(w http.ResponseWriter, req *http.Request, _ map[string]string) {
			h.ServeHTTP(w, req)
		}); err != nil {
			return err
		}
	}
	return nil
}
