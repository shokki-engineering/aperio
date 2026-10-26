# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# The ota database's migrations, owned by Prisma: the schema is the generated
# protobuf/generated/prisma/ota (`just gen prisma`), and its migrations/ is the
# history. Invoked as `just db <recipe>`. The database is OTA_DATABASE_URL, read
# from protobuf/generated/prisma/ota/.env (copy .env.example).
set working-directory := '../../protobuf/generated/prisma/ota'

# List recipes.
[group('db')]
default:
    @just --list db

# Install the Prisma CLI the generated package.json pins.
[group('db')]
install:
    npm install --no-audit --no-fund

# The local Postgres the migrations apply to (127.0.0.1:5433, database ota).
[group('db')]
up:
    docker compose -f ../../../../deploy/compose/compose.yaml up -d --wait

# Write the next migration after the schema changes, and apply it.
[group('db')]
migrate name:
    npx prisma migrate dev --name {{ name }} --config ota.config.ts

# Apply pending migrations, writing none: CI and deployments.
[group('db')]
deploy:
    npx prisma migrate deploy --config ota.config.ts

# Which migrations the database has and has not seen.
[group('db')]
status:
    npx prisma migrate status --config ota.config.ts

# Fail when the generated schema is not a valid Prisma schema.
[group('db')]
validate:
    npx prisma validate --config ota.config.ts
