// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// mcp_test.go checks the MCP surface: a client's Authorization header, sent
// with each HTTP request, reaches the interceptor for that call -- and only
// that call, in a session that changes it -- through the runtime's header
// mappings; without them, no token arrives and staff tools are refused.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// header is a RoundTripper setting Authorization to whatever value holds.
type header struct{ value *string }

// RoundTrip sends req with the current Authorization.
func (h header) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if *h.value != "" {
		req.Header.Set("Authorization", *h.value)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// getVenue calls the GetVenue tool, returning its text and whether it
// failed.
func getVenue(t *testing.T, s *mcp.ClientSession) (string, bool) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "venue_service-get_venue_v1", Arguments: map[string]any{"name": "venues/lakeside"},
	})
	if err != nil {
		return err.Error(), true
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return text.String(), res.IsError
}

// connect is a client session to h with the Authorization in value.
func connect(t *testing.T, h http.Handler, value *string) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	s, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: srv.URL + "/mcp", HTTPClient: &http.Client{Transport: header{value}}, MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestMCPCarriesTheToken calls a tool without, then with, a token in one
// session.
func TestMCPCarriesTheToken(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	value := ""
	s := connect(t, mcpHandler(t, a, true), &value)
	if text, failed := getVenue(t, s); !failed {
		t.Errorf("no token: %s, want refused", text)
	}
	value = "Bearer " + iss.staff(t, []string{"lakeside"}, "front_desk")
	if text, failed := getVenue(t, s); failed || !strings.Contains(text, "john.doe@example.com") {
		t.Errorf("lakeside's desk: %s (failed %v)", text, failed)
	}
	value = ""
	if text, failed := getVenue(t, s); !failed {
		t.Errorf("token dropped mid-session: %s, want refused", text)
	}
}

// TestMCPWithoutHeaderMappings shows what the mappings do: without them the
// header never reaches the interceptor, so even a valid token is refused.
func TestMCPWithoutHeaderMappings(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	value := "Bearer " + iss.staff(t, []string{"lakeside"}, "front_desk")
	s := connect(t, mcpHandler(t, a, false), &value)
	if text, failed := getVenue(t, s); !failed {
		t.Errorf("without header mappings: %s, want refused", text)
	}
}
