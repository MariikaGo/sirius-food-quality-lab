#!/bin/zsh
set -e
ROOT="${0:A:h:h}"
cd "$ROOT"
mkdir -p .runtime
find_pg() {
  for candidate in /opt/homebrew/opt/postgresql@{14,15,16,17,18}/bin/$1; do
    if [[ -x "$candidate" ]]; then print -r -- "$candidate"; return; fi
  done
  command -v "$1"
}
PG_CTL="$(find_pg pg_ctl)"
INITDB="$(find_pg initdb)"
PSQL="$(find_pg psql)"
CREATEDB="$(find_pg createdb)"
PG_ISREADY="$(find_pg pg_isready)"
REDIS_SERVER="$(command -v redis-server || true)"
REDIS_CLI="$(command -v redis-cli || true)"
[[ -n "$REDIS_SERVER" ]] || { print 'Установите Redis: brew install redis'; exit 1; }
if [[ "${1:-start}" == stop ]]; then
 if [[ -f .runtime/redis.pid ]]; then "$REDIS_CLI" -p 56379 shutdown; fi
 if [[ -d .runtime/postgres ]]; then "$PG_CTL" -D "$ROOT/.runtime/postgres" -m fast stop || true; fi
 exit 0
fi
if [[ ! -d .runtime/postgres ]]; then "$INITDB" -D "$ROOT/.runtime/postgres" -A trust --no-locale -E UTF8; fi
if "$PG_ISREADY" -h 127.0.0.1 -p 55432 >/dev/null 2>&1; then
 "$PG_CTL" -D "$ROOT/.runtime/postgres" status >/dev/null || { print 'Порт 55432 занят другой работой. Сначала запустите её stop.command.'; exit 1; }
else
 "$PG_CTL" -D "$ROOT/.runtime/postgres" -l "$ROOT/.runtime/postgres.log" -o '-h 127.0.0.1 -p 55432 -k /private/tmp' start
fi
if [[ "$("$PSQL" -h 127.0.0.1 -p 55432 -d postgres -Atc "SELECT 1 FROM pg_roles WHERE rolname='sirius'")" != 1 ]]; then
 "$PSQL" -h 127.0.0.1 -p 55432 -d postgres -c "CREATE ROLE sirius LOGIN PASSWORD 'sirius'"
fi
for dbname in sirius_go sirius_go_test sirius_go_bench; do
 if [[ "$("$PSQL" -h 127.0.0.1 -p 55432 -d postgres -Atc "SELECT 1 FROM pg_database WHERE datname='$dbname'")" != 1 ]]; then
  "$CREATEDB" -h 127.0.0.1 -p 55432 -O sirius "$dbname"
 fi
done
if "$REDIS_CLI" -p 56379 ping >/dev/null 2>&1; then
 [[ -f .runtime/redis.pid ]] || { print 'Порт 56379 занят другой работой. Сначала запустите её stop.command.'; exit 1; }
else
 "$REDIS_SERVER" --bind 127.0.0.1 --port 56379 --dir "$ROOT/.runtime" --daemonize yes --pidfile "$ROOT/.runtime/redis.pid" --logfile "$ROOT/.runtime/redis.log"
fi
