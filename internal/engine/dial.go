// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// dial.go connects to freebusy from aperio's configuration: the endpoint,
// TLS (the system's roots, a CA file, or none), and a timeout per call.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/freebusy-go/option"
)

// DialConfig connects to freebusy as the [freebusy] configuration says. The SDK reads
// FREEBUSY_ENDPOINT and its other variables for anything left unset here.
func DialConfig(c config.FreebusyConfig) (*Client, error) {
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
	return Dial(context.Background(), opts...)
}
