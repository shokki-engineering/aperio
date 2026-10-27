# syntax=docker/dockerfile:1.7
# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# aperio's two images, from one file (`just image` builds both):
#
#   runtime   the server: the static binary on distroless, nothing else. The
#             default target. Configured entirely by APERIO_<SECTION>__<KEY>
#             variables over the release defaults compiled into it
#             (internal/config/aperio.release.toml).
#   migrate   Prisma and the aperio database's migrations: `prisma migrate
#             deploy` against APERIO_DATABASE_URL, applying only what the
#             database has not seen, then exiting. How a deployment with no
#             checkout -- the Compose stack, the Helm chart's pre-install Job --
#             runs `just db deploy`.
#
#   docker build -t aperio .
#   docker build --target migrate -t aperio-migrate .
#
# A release stamps its tag and commit (build.go):
#   --build-arg VERSION=v0.1.0 --build-arg COMMIT=$(git rev-parse HEAD)

ARG GO_VERSION=1.26.10
ARG NODE_VERSION=22.23.4

# The binary, cross-compiled on the build machine for the target platform:
# CGO off, so it is static and runs on an image with no libc.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=""
ARG COMMIT=""
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X main.buildVersion=${VERSION} -X main.buildCommit=${COMMIT}" \
      -o /out/aperio .

# The migrations: the Prisma CLI the generated package.json resolves to
# (pinned, so every image applies them the same way), the generated schema
# and its migrations/ history. Debian, not Alpine: Prisma's schema engine
# wants glibc and OpenSSL.
FROM node:${NODE_VERSION}-bookworm-slim AS migrate
ARG PRISMA_VERSION=7.10.1
ARG DOTENV_VERSION=16.6.1
RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
ENV CHECKPOINT_DISABLE=1 \
    PRISMA_HIDE_UPDATE_MESSAGE=1 \
    NODE_ENV=production
WORKDIR /prisma
RUN chown node:node /prisma
USER node
RUN npm init -y >/dev/null \
    && npm install --no-audit --no-fund --save-exact prisma@${PRISMA_VERSION} dotenv@${DOTENV_VERSION} \
    && npm cache clean --force
COPY --chown=node:node protobuf/generated/prisma/aperio/ ./
LABEL org.opencontainers.image.title="aperio-migrate" \
      org.opencontainers.image.description="The aperio database's Prisma migrations, applied with prisma migrate deploy." \
      org.opencontainers.image.source="https://github.com/shokki-engineering/aperio" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later"
# The database is APERIO_DATABASE_URL: postgresql://user:password@host:5432/aperio.
ENTRYPOINT ["/prisma/node_modules/.bin/prisma", "migrate", "deploy", "--config", "aperio.config.ts"]

# The server: no shell, no package manager; the binary carries the time zone
# database and fallback root certificates itself (main.go). Not root.
FROM gcr.io/distroless/static-debian13:nonroot AS runtime
COPY --from=build /out/aperio /aperio
LABEL org.opencontainers.image.title="aperio" \
      org.opencontainers.image.description="Venue backend: stays, events, leads, quotes, GST invoices and payments, booked through freebusy." \
      org.opencontainers.image.source="https://github.com/shokki-engineering/aperio" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later"
USER 65532:65532
# gRPC, HTTP (REST and /health), MCP (/mcp).
EXPOSE 50061 8090 8091
# No HEALTHCHECK: the image has no shell or HTTP client to run one. Probe
# GET /health on 8090 from outside (Compose, Kubernetes).
ENTRYPOINT ["/aperio"]
