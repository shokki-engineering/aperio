// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package telemetry holds aperio's process-wide telemetry and configuration
// state. Importing it does nothing: the server builds the telemetry pipeline
// with Init at startup, and a program embedding aperio as a library either
// hands it a pipeline of its own (Use) or runs without one.
package telemetry

import (
	_ "embed"
	"fmt"
	"os"
	"sync"

	"github.com/the-protobuf-project/runtime-go/observability"
	"github.com/the-protobuf-project/telemetry/telemetry-go"
)

//go:embed telemetry.toml
var telemetryConfigData []byte

var (
	// Telemetry is the process's telemetry pipeline: set by Init (the
	// server) or Use (a library caller), nil until then. Nil means none --
	// no spans, no metrics, no logs -- and every reader in aperio allows
	// for it.
	Telemetry *telemetry.Telemetry
	once      sync.Once // once builds the server's pipeline at most once.
	initErr   error
)

// Init builds the server's telemetry pipeline from the embedded
// telemetry.toml and the APERIO_ deployment overrides, makes it the
// process's (Telemetry), and has runtime-go's modules -- the gRPC server, its
// otelgrpc spans and gRPC metrics, the cache -- log, trace and measure through
// it instead of each standing up an exporter of its own. The server calls it
// first thing at startup; later calls return the first result.
func Init() error {
	once.Do(func() {
		b := telemetry.New()
		if tmpPath, err := writeTelemetryConfig(); err == nil {
			defer os.Remove(tmpPath) //nolint:errcheck
			b.WithConfig(tmpPath)
		} else {
			// No writable temp directory: a scratch or read-only container.
			// The library reads its configuration only from a file, so the
			// service is named in code, from the same embedded file, and the
			// rest is the library's defaults -- OTLP export included, which
			// APERIO_OTLP_ENDPOINT turns on.
			name, version, description, err := telemetryService()
			if err != nil {
				initErr = fmt.Errorf("read embedded telemetry config: %w", err)
				return
			}
			b.WithService(name, version).WithDescription(description)
		}
		b.WithTracing()
		applyDeploymentOverrides(b)

		o, err := b.Build()
		if err != nil {
			initErr = fmt.Errorf("create telemetry: %w", err)
			return
		}
		Use(o)
		observability.Adopt(o)
	})
	return initErr
}

// Use makes t the process's telemetry pipeline: aperio's RPC spans and
// metrics, its database instrumentation and its own log lines go through it.
// Nil turns them off.
func Use(t *telemetry.Telemetry) {
	Telemetry = t
}

// Info logs a line through the process's pipeline, if there is one. kv is
// key/value pairs, or a single map.
func Info(msg string, kv ...any) {
	if t := Telemetry; t != nil {
		t.Logger.Info(msg, fields(kv)...)
	}
}

// Debug logs a debug line through the process's pipeline, if there is one.
// kv is key/value pairs, or a single map.
func Debug(msg string, kv ...any) {
	if t := Telemetry; t != nil {
		t.Logger.Debug(msg, fields(kv)...)
	}
}

// Warn logs a warning through the process's pipeline, if there is one. kv is
// key/value pairs, or a single map.
func Warn(msg string, kv ...any) {
	if t := Telemetry; t != nil {
		t.Logger.Warn(msg, fields(kv)...)
	}
}

// Error logs an error through the process's pipeline, if there is one. kv is
// key/value pairs, or a single map.
func Error(msg string, kv ...any) {
	if t := Telemetry; t != nil {
		_ = t.Logger.Error(msg, fields(kv)...)
	}
}

// fields is kv as the pipeline's logger takes it: one value, of which it
// logs only the first. Key/value pairs become one map, so none is dropped;
// a single value is passed as it is, and an odd trailing value is kept
// under "extra".
func fields(kv []any) []any {
	if len(kv) <= 1 {
		return kv
	}
	m := make(map[string]any, (len(kv)+1)/2)
	for i := 0; i < len(kv); i += 2 {
		if i+1 == len(kv) {
			m["extra"] = kv[i]
			break
		}
		v := kv[i+1]
		if err, ok := v.(error); ok {
			v = err.Error()
		}
		m[fmt.Sprint(kv[i])] = v
	}
	return []any{m}
}

// Close releases resources held by the Telemetry client.
// It is safe to call even when Telemetry was never initialised.
// The caller (typically main) should invoke this on application shutdown.
func Close() error {
	if Telemetry != nil {
		return Telemetry.Close()
	}
	return nil
}
