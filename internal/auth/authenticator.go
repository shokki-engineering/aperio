// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// authenticator.go builds what the interceptor needs from the [auth]
// configuration: the issuer's verifier, the policy and the link signer.
// Without an issuer auth is off -- every call goes through -- and the
// startup says so as loudly as it can.

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// Authenticator checks calls against one issuer and one policy.
type Authenticator struct {
	verifier *Verifier // nil when auth is off
	policy   Policy
	links    *Links
}

// New is the authenticator cfg describes. It discovers the issuer, so it
// fails when the issuer cannot be reached. With no issuer configured it
// returns one that lets every call through, after a loud warning.
func New(ctx context.Context, cfg config.AuthConfig) (*Authenticator, error) {
	return newAuthenticator(ctx, cfg, nil)
}

// newAuthenticator is New on the clock now (nil for time.Now).
func newAuthenticator(ctx context.Context, cfg config.AuthConfig, now func() time.Time) (*Authenticator, error) {
	secret, err := linkSecret(cfg.LinkSecret)
	if err != nil {
		return nil, err
	}
	a := &Authenticator{policy: DefaultPolicy(), links: NewLinks(secret, now)}
	if !cfg.Enabled() {
		loud("AUTH IS OFF: auth.issuer is not set, so every call on gRPC, REST and MCP is let through unchecked. " +
			"Set APERIO_AUTH__ISSUER and APERIO_AUTH__AUDIENCE before this server faces a network.")
		return a, nil
	}
	if a.verifier, err = NewVerifier(ctx, cfg, now); err != nil {
		return nil, err
	}
	if missing := a.policy.Unguarded(); len(missing) > 0 {
		telemetry.Error("auth: methods with no access policy are refused to all but an unlimited owner",
			"methods", strings.Join(missing, ", "))
	}
	return a, nil
}

// Enabled reports whether calls are checked.
func (a *Authenticator) Enabled() bool { return a.verifier != nil }

// Links is the signer of guests' links, for a service that sends one.
func (a *Authenticator) Links() *Links { return a.links }

// linkSecret is the configured link secret, or one made for this process
// when none is: its links then outlive neither a restart nor a move to
// another replica, which is said.
func linkSecret(configured string) ([]byte, error) {
	if configured != "" {
		return []byte(configured), nil
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	telemetry.Warn("auth.link_secret is not set: guests' links are this process's own; set APERIO_AUTH__LINK_SECRET")
	return secret, nil
}

// loud warns through telemetry and on standard error, which a warning can't
// be missed on even before telemetry is up.
func loud(msg string) {
	telemetry.Warn(msg)
	fmt.Fprintln(os.Stderr, "WARNING: "+msg)
}
