// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// deployment.go is what varies by deployment: the environment, the OTLP
// collector and the site labels, read from APERIO_ variables over the
// embedded telemetry.toml, so one binary serves every deployment.

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/the-protobuf-project/telemetry/telemetry-go"
	"github.com/the-protobuf-project/telemetry/telemetry-go/options"
)

// deploymentEnvironment is the environment telemetry reports: the
// APERIO_TELEMETRY_ENVIRONMENT override, else APERIO_SERVER__ENVIRONMENT,
// else the configuration profile's -- "production" when the release defaults
// stand alone (APERIO_PROFILE=release, or no development profile in the
// working directory), empty (telemetry.toml's "development") when the
// development profile applies. Without the profile check a release build said
// "development" in its banner and on every log line. config cannot be read
// here (it imports this package), so its profile rule is mirrored; see
// internal/config/load.go.
func deploymentEnvironment() string {
	if env := os.Getenv("APERIO_TELEMETRY_ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("APERIO_SERVER__ENVIRONMENT"); env != "" {
		return env
	}
	switch os.Getenv("APERIO_PROFILE") {
	case "release":
		return "production"
	case "dev":
		return ""
	}
	if _, err := os.Stat("internal/config/aperio.dev.toml"); err != nil {
		return "production"
	}
	return ""
}

// applyDeploymentOverrides layers per-deployment values over the embedded
// telemetry.toml defaults, so one binary serves every environment and
// deployment without a rebuild. Each override is optional — an unset
// variable leaves the embedded default in place.
//
//	APERIO_TELEMETRY_ENVIRONMENT  deployment environment (development|staging|production)
//	APERIO_OTLP_ENDPOINT          OTLP collector as host:port
//	APERIO_OTLP_SECURE            true when that collector serves TLS; unset, TLS
//	                                is on only for a remote host on 443 or 4318
//	APERIO_SITE                   label slug for what this deployment serves (e.g. "riverside-club")
//	APERIO_SITE_NAME              human-readable name for it
//	APERIO_TELEMETRY_INSTANCE_ID  service.instance.id; unset, the host name
//	                                (a pod's name in Kubernetes)
//
// The resource also carries host.name, and whatever OTEL_RESOURCE_ATTRIBUTES
// lists (the SDK's resource.Default reads it): the Helm chart passes the pod's
// k8s.pod.name, k8s.namespace.name and k8s.node.name through it. Without an
// instance id, replicas exporting the same service.name overwrite each other's
// metric series.
func applyDeploymentOverrides(b *telemetry.Builder) {
	if env := deploymentEnvironment(); env != "" {
		b.WithEnvironment(options.Environment(env))
	}
	if endpoint := os.Getenv("APERIO_OTLP_ENDPOINT"); endpoint != "" {
		if host, port, ok := splitHostPort(endpoint); ok {
			b.WithOTLP(host, port)
		} else {
			fmt.Printf("WARNING: ignoring malformed APERIO_OTLP_ENDPOINT %q (want host:port)\n", endpoint)
		}
	}
	if secure, err := strconv.ParseBool(os.Getenv("APERIO_OTLP_SECURE")); err == nil {
		b.WithOTLPSecure(secure)
	}
	labels := instanceLabels()
	if v := os.Getenv("APERIO_SITE"); v != "" {
		labels["site"] = v
	}
	if v := os.Getenv("APERIO_SITE_NAME"); v != "" {
		labels["site_name"] = v
	}
	if len(labels) > 0 {
		b.WithLabels(labels)
	}
}

// instanceLabels returns the resource attributes that tell one running
// process from another: service.instance.id (APERIO_TELEMETRY_INSTANCE_ID,
// else the host name) and host.name.
func instanceLabels() map[string]string {
	labels := map[string]string{}
	host, _ := os.Hostname()
	if host != "" {
		labels["host.name"] = host
	}
	id := os.Getenv("APERIO_TELEMETRY_INSTANCE_ID")
	if id == "" {
		id = host
	}
	if id != "" {
		labels["service.instance.id"] = id
	}
	return labels
}

// splitHostPort splits a "host:port" OTLP endpoint into its parts.
func splitHostPort(endpoint string) (host string, port int, ok bool) {
	h, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", 0, false
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, false
	}
	return h, p, true
}

// writeTelemetryConfig writes the embedded telemetry.toml to a temporary
// file and returns its path. The caller is responsible for cleanup.
func writeTelemetryConfig() (string, error) {
	f, err := os.CreateTemp("", "telemetry-*.toml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := f.Write(telemetryConfigData); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write telemetry config: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return f.Name(), nil
}

// telemetryService reads the service's identity from the embedded
// telemetry.toml, for when the library cannot be handed the file.
func telemetryService() (name, version, description string, err error) {
	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(telemetryConfigData), toml.Parser()); err != nil {
		return "", "", "", err
	}
	return k.String("service.name"), k.String("service.version"), k.String("service.description"), nil
}
