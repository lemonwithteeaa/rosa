#!/usr/bin/env bash
set -euo pipefail
VERSION="${1:-759}"
OUT="${2:-../РОСА_7.3.11.${VERSION}-RC.exe}"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '-H windowsgui' -o "$OUT" .
file "$OUT" | grep -q 'PE32+ executable'
objdump -h "$OUT" | grep -q '\.rsrc'
objdump -h "$OUT" | grep -Eq '\.(zdebug_info|debug_info)'
objdump -h "$OUT" | grep -Eq '\.(zdebug_line|debug_line)'
