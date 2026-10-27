# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# The k6 load test, test/load/aperio.js: staff reads, guest bookings (real
# freebusy holds), in-house requests and MCP agents at once. Invoked as
# `just load <profile>`; recipes run from the repo root.
#
# It needs aperio AND freebusy: `just stack up` (or `just stack up cache` to
# measure with the Redis caches on). setup() makes a fresh venue each run,
# with ROOMS rooms per room type as freebusy resources. Each run's summary is
# written to test/load/results/ (git-ignored).
set working-directory := '../..'

# Where the stack answers (the Compose defaults).
aperio := env_var_or_default("APERIO_URL", "http://localhost:8090")
mcp := env_var_or_default("MCP_URL", "http://localhost:8091/mcp")
freebusy := env_var_or_default("FREEBUSY_URL", "http://localhost:8080")

# List recipes.
[group('load')]
default:
    @just --list load

# Every endpoint once per scenario, with the busy-day thresholds: a CI gate.
[group('load')]
smoke: (run "smoke" "1" "" "5")

# A busy day, steady, for 3 minutes (or duration): the budget run.
[group('load')]
day rate="1" duration="": (run "load" rate duration "40")

# Ramping to five times a busy day: where aperio stops keeping up.
[group('load')]
stress rate="1" duration="": (run "stress" rate duration "60")

# A busy day for 30 minutes (or duration): leaks, pool exhaustion, drift.
[group('load')]
soak rate="1" duration="": (run "soak" rate duration "60")

# One run of profile at rate times a busy day, rooms per room type.
[group('load')]
run profile rate="1" duration="" rooms="40":
    mkdir -p test/load/results
    k6 run -e PROFILE={{ profile }} -e RATE={{ rate }} -e DURATION={{ duration }} -e ROOMS={{ rooms }} \
      -e APERIO_URL={{ aperio }} -e MCP_URL={{ mcp }} -e FREEBUSY_URL={{ freebusy }} \
      --summary-export test/load/results/{{ profile }}-$(date +%Y%m%d-%H%M%S).json \
      test/load/aperio.js
