// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// runtime.go opens what the services run on, from the configuration: the
// database connection (opened here when the caller hands none, as the e2e
// suites do) and the freebusy client.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/freebusy-go/option"
)

// newServices assembles the services on conn, or on a connection opened
// from config when conn is nil, with the freebusy client the config names.
// It returns the client too, which the server closes on Stop.
func newServices(conn *database.Connection) (*Services, *engine.Client, error) {
	if conn == nil {
		opened, err := database.Open()
		if err != nil {
			return nil, nil, err
		}
		conn = opened
	}
	fb, err := dialFreebusy(config.Get().Freebusy)
	if err != nil {
		return nil, nil, err
	}
	return New(conn, fb), fb, nil
}

// dialFreebusy connects the freebusy SDK as [freebusy] says. The SDK reads
// FREEBUSY_ENDPOINT and its other variables for anything left unset here.
func dialFreebusy(c config.FreebusyConfig) (*engine.Client, error) {
	var opts []option.RequestOption
	if c.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(c.Endpoint))
	}
	if c.TimeoutSeconds > 0 {
		opts = append(opts, option.WithTimeout(time.Duration(c.TimeoutSeconds)*time.Second))
	}
	switch {
	case c.Insecure:
		opts = append(opts, option.WithInsecure())
	case c.CAFile != "":
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("freebusy.ca_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("freebusy.ca_file %s holds no PEM certificate", c.CAFile)
		}
		opts = append(opts, option.WithTLSConfig(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}))
	}
	return engine.Dial(context.Background(), opts...)
}
