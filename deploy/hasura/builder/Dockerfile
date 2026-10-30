# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# The DDN CLI in a container, to build the supergraph -- aperio's subgraph and
# freebusy's, which arrives in a Docker volume and is never on the host (see
# compose.yaml, `supergraph`). Hasura publishes no CLI image, and only an
# amd64 Linux binary (statically linked), so this runs as linux/amd64; the
# checksum pins the download, as there is no published one to check.
#
# The CA certificates come from Alpine: the CLI checks its login with
# Hasura's API even for a local build.
FROM --platform=linux/amd64 alpine:3.22@sha256:2e0b81d60597f449baa56dc25d355367869d2dca4cd57e866f1e2b52af3eb55f AS certs

FROM --platform=linux/amd64 busybox:1.37.0@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ADD --chmod=755 --checksum=sha256:ffde99f15f18e26f6953f1eb21dc1fd62c3cb3a08a49108a21632b14d2d31fda \
    https://graphql-engine-cdn.hasura.io/ddn/cli/v4/v3.9.4/cli-ddn-linux-amd64 /usr/local/bin/ddn
WORKDIR /project
ENTRYPOINT ["ddn"]
