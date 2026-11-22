#!/bin/sh
set -eu
cd "$(dirname "$0")"
if ! docker info >/dev/null 2>&1; then
  orbctl start
fi
if docker image inspect sirius-practice10:local >/dev/null 2>&1; then
  docker compose up -d --no-build --wait
else
  docker compose up -d --build --wait
fi
printf '\nSwagger: http://127.0.0.1:8000/docs\nМонитор: http://127.0.0.1:8000/monitor/\n'
