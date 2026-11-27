#!/bin/sh
set -eu
cd "$(dirname "$0")"
if ! docker info >/dev/null 2>&1; then orbctl start; fi
python3 scripts/bootstrap.py
docker compose up -d --build --wait --wait-timeout 600
printf "API: http://127.0.0.1:8000/docs\n"
