#!/usr/bin/env bash
set -euo pipefail
src=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(dirname -- "$src")
mkdir -p -- "$root"/{run,.cache/go,tmp}
export GOCACHE="$root/.cache/go" GOTMPDIR="$root/tmp"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off
cd -- "$src"
printf 'Building offline from cached Go modules. No engine will start.\n'
name=llm-tui
if [[ $(go env GOOS) == windows ]]; then name+=.exe; fi
go build -mod=readonly -trimpath -buildvcs=false -o "$root/run/$name" main.go
if [[ ! -e "$root/run/catalog.json" && ! -L "$root/run/catalog.json" ]]; then
    cp -n -- examples/catalog.json "$root/run/catalog.json"
fi
printf 'Built run/%s; catalog: run/catalog.json\n' "$name"
