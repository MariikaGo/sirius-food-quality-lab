#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p evidence
go test -count=1 -v ./... 2>&1 | tee evidence/testcontainers-tests.txt
