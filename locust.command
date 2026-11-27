#!/bin/sh
set -eu
cd "$(dirname "$0")"
LOAD_UI=1 python3 scripts/load.py
