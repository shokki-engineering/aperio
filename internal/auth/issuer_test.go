// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// issuer_test.go is an OIDC issuer for the tests: an httptest server
// answering discovery and JWKS with an RSA key made for the run, and a
// signer of tokens from it.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/shokki-engineering/aperio/internal/config"
)

// testAudience is the client ID the test issuer's tokens are for.
const testAudience = "aperio-test"

// issuer is a running test issuer.
type issuer struct {
	URL    string
	signer jose.Signer
}

// newIssuer starts an issuer, closed when t ends.
func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithHeader("kid", "k1").WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": srv.URL, "jwks_uri": srv.URL + "/jwks",
			"authorization_endpoint": srv.URL + "/auth", "token_endpoint": srv.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
		}})
	})
	return &issuer{URL: srv.URL, signer: signer}
}

// token is a signed token for John Doe with roles and venues, expiring at
// exp; extra claims override the defaults.
func (i *issuer) token(t *testing.T, roles, venues []string, exp time.Time, extra map[string]any) string {
	t.Helper()
	claims := map[string]any{
		"iss": i.URL, "sub": "john-doe", "aud": testAudience, "email": "john.doe@example.com",
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": exp.Unix(),
		"roles": roles, "venues": venues,
	}
	for k, v := range extra {
		claims[k] = v
	}
	body, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	jws, err := i.signer.Sign(body)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// staff is a valid token, for an hour, with roles and venues.
func (i *issuer) staff(t *testing.T, venues []string, roles ...string) string {
	return i.token(t, roles, venues, time.Now().Add(time.Hour), nil)
}

// config is the [auth] block for this issuer.
func (i *issuer) config() config.AuthConfig {
	return config.AuthConfig{Issuer: i.URL, Audience: testAudience, LinkSecret: "test-link-secret"}
}

// authenticator is an Authenticator over this issuer.
func (i *issuer) authenticator(t *testing.T) *Authenticator {
	t.Helper()
	a, err := New(context.Background(), i.config())
	if err != nil {
		t.Fatal(err)
	}
	return a
}
