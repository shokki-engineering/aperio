# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# The aperio database's migrations, owned by Prisma: the schema is the generated
# protobuf/generated/prisma/aperio (`just gen prisma`), and its migrations/ is the
# history. Invoked as `just db <recipe>`. The database is APERIO_DATABASE_URL, read
# from protobuf/generated/prisma/aperio/.env (copy .env.example).
set working-directory := '../../protobuf/generated/prisma/aperio'

# List recipes.
[group('db')]
default:
    @just --list db

# Install the Prisma CLI the generated package.json pins.
[group('db')]
install:
    npm install --no-audit --no-fund

# The local Postgres the migrations apply to (127.0.0.1:5433, database aperio).
[group('db')]
up:
    docker compose -f ../../../../deploy/compose/compose.yaml up -d --wait

# Write the next migration after the schema changes, and apply it.
[group('db')]
migrate name:
    npx prisma migrate dev --name {{ name }} --config aperio.config.ts

# Apply pending migrations, writing none: CI and deployments.
[group('db')]
deploy:
    npx prisma migrate deploy --config aperio.config.ts

# Which migrations the database has and has not seen.
[group('db')]
status:
    npx prisma migrate status --config aperio.config.ts

# Fail when the generated schema is not a valid Prisma schema.
[group('db')]
validate:
    npx prisma validate --config aperio.config.ts
