# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# The whole system in Compose (deploy/compose/compose.yaml): aperio, freebusy,
# both Postgres and their migrations; by profile both Hasura engines
# (hasura), Redis (cache) and OpenSearch (search). Settings come from
# deploy/compose/.env (copy .env.example). Invoked as `just stack <recipe>`:
#   just stack up                         # the core stack
#   just stack up cache search hasura     # with profiles
set working-directory := '../../deploy/compose'

# List recipes.
[group('stack')]
default:
    @just --list stack

# Start the stack with the named profiles, building aperio's images when missing; waits for health.
[group('stack')]
up *profiles:
    docker compose {{ prepend('--profile ', profiles) }} up -d --wait

# Stop the stack, keeping its volumes.
[group('stack')]
down:
    docker compose --profile '*' down

# DEV ONLY: stop the stack and delete its volumes -- both databases with every row.
[group('stack')]
[confirm('Delete the stack volumes, both databases included? (y/N)')]
destroy:
    docker compose --profile '*' down -v

# Follow the logs of every service, or of those named.
[group('stack')]
logs *services:
    docker compose --profile '*' logs -f {{ services }}

# The stack's containers and their state.
[group('stack')]
ps:
    docker compose --profile '*' ps -a

# Check the compose file and .env for every profile.
[group('stack')]
config:
    docker compose --profile '*' config --quiet
