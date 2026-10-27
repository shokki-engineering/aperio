# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# The Helm chart, deploy/helm/aperio. Invoked as `just helm <recipe>`; CI's
# deploy job runs `just helm check`. Recipes run from the repo root.
#
# The chart needs database.postgres.host and a Postgres password, so it is
# rendered with a values file from its ci/ folder: minimal (the least a
# release takes) or full (every optional part on).
set working-directory := '../..'

chart := 'deploy/helm/aperio'
kubeconform := 'ghcr.io/yannh/kubeconform:v0.7.0'
# Kubernetes' own schemas, then the CRD catalog (the ServiceMonitor).
crds := 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json'

# List recipes.
[group('helm')]
default:
    @just --list helm

# helm lint --strict, with the minimal and the full values.
[group('helm')]
lint:
    helm lint --strict {{ chart }} -f {{ chart }}/ci/minimal-values.yaml
    helm lint --strict {{ chart }} -f {{ chart }}/ci/full-values.yaml

# Render the chart with ci/<values>-values.yaml (minimal or full).
[group('helm')]
template values="minimal":
    helm template aperio {{ chart }} -f {{ chart }}/ci/{{ values }}-values.yaml

# Check the rendered manifests against the Kubernetes schemas (kubeconform,
# in Docker), with both values files.
[group('helm')]
validate:
    #!/usr/bin/env bash
    set -euo pipefail
    for v in minimal full; do
      echo "==> $v"
      helm template aperio {{ chart }} -f {{ chart }}/ci/$v-values.yaml |
        docker run --rm -i {{ kubeconform }} -strict -summary \
          -schema-location default -schema-location '{{ crds }}'
    done

# The values schema refuses what it should: each bad value must fail.
[group('helm')]
schema:
    #!/usr/bin/env bash
    set -euo pipefail
    for bad in search.provider=elastic replicaCount=-1 secrets.conciergeHoldSecret=short \
        database.postgres.sslmode=maybe unknownKey=1 notify.provider=sms; do
      if helm template aperio {{ chart }} -f {{ chart }}/ci/minimal-values.yaml --set "$bad" >/dev/null 2>&1; then
        echo "values.schema.json accepted $bad" >&2; exit 1
      fi
    done
    echo "values.schema.json refused every bad value"

# Lint, schema and kubeconform: what CI runs.
[group('helm')]
check: lint schema validate

# Package the chart into dist/, at version (default: Chart.yaml's).
[group('helm')]
package version="":
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{ version }}" ]; then
      helm package {{ chart }} --version "{{ version }}" --app-version "{{ version }}" -d dist
    else
      helm package {{ chart }} -d dist
    fi
