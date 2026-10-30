{{/* Copyright 2026 Srikanth K. */}}
{{/* SPDX-License-Identifier: AGPL-3.0-or-later */}}

{{- define "aperio.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else if contains .Chart.Name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "aperio.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "aperio.selector" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/* The server pods: what the Service, PDB, HPA and NetworkPolicy select. */}}
{{- define "aperio.serverSelector" -}}
{{ include "aperio.selector" . }}
app.kubernetes.io/component: server
{{- end -}}

{{- define "aperio.secretName" -}}
{{- .Values.secrets.existingSecret | default (include "aperio.fullname" .) -}}
{{- end -}}

{{- define "aperio.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- .Values.serviceAccount.name | default (include "aperio.fullname" .) -}}
{{- else -}}
{{- .Values.serviceAccount.name | default "default" -}}
{{- end -}}
{{- end -}}

{{/* repository:tag, or repository@digest when a digest is pinned. The tag
defaults to the chart's appVersion. Call with (dict "image" ... "root" $). */}}
{{- define "aperio.image" -}}
{{- if .image.digest -}}
{{- printf "%s@%s" .image.repository .image.digest -}}
{{- else -}}
{{- printf "%s:%s" .image.repository (.image.tag | default .root.Chart.AppVersion) -}}
{{- end -}}
{{- end -}}

{{/* The replicas the PDB protects: the HPA's floor when it scales. */}}
{{- define "aperio.minReplicas" -}}
{{- if .Values.autoscaling.enabled -}}
{{- .Values.autoscaling.minReplicas -}}
{{- else -}}
{{- .Values.replicaCount -}}
{{- end -}}
{{- end -}}

{{/* Unprivileged, read-only: the server image holds a static binary. */}}
{{- define "aperio.securityContext" -}}
runAsNonRoot: true
runAsUser: 65532
runAsGroup: 65532
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop: ["ALL"]
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{- define "aperio.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 65532
runAsGroup: 65532
fsGroup: 65532
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{/* A secret key as an env var's source. Every key but postgres-password is
optional: a chart-made Secret holds only the values set, and an existing one
holds what its owner put in it. Call with (dict "name" ... "secret" ... "key" ...). */}}
{{- define "aperio.secretEnv" -}}
- name: {{ .name }}
  valueFrom:
    secretKeyRef: { name: {{ .secret }}, key: {{ .key }}, optional: true }
{{- end -}}

{{/*
Booking.com's hotels as JSON for APERIO_CHANNELS__BOOKINGCOM__HOTELS_JSON,
with every hotel and room code a string: --set, or an unquoted code in a
values file, makes "7777001" a number, which aperio's config refuses. A
number is written whole (YAML reads 7777001 as 7.777001e+06); a code with
leading zeros must still be quoted, or YAML drops them.
*/}}
{{- define "aperio.bookingcomHotels" -}}
{{- $hotels := list -}}
{{- range . -}}
{{- $h := deepCopy . -}}
{{- $_ := set $h "hotel_code" (include "aperio.code" $h.hotel_code) -}}
{{- $rooms := list -}}
{{- range ($h.rooms | default list) -}}
{{- $r := deepCopy . -}}
{{- $_ := set $r "code" (include "aperio.code" $r.code) -}}
{{- $rooms = append $rooms $r -}}
{{- end -}}
{{- if $h.rooms }}{{ $_ := set $h "rooms" $rooms }}{{ end -}}
{{- $hotels = append $hotels $h -}}
{{- end -}}
{{- toJson $hotels -}}
{{- end -}}

{{/* A code as a string: a number written without exponent or decimals. */}}
{{- define "aperio.code" -}}
{{- if kindIs "string" . }}{{ . }}{{ else }}{{ int64 . }}{{ end -}}
{{- end -}}
