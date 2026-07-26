#!/bin/sh
set -eu

mode=${1-}
scope=${2-}

case "$mode" in
  write|check) ;;
  *)
    printf 'usage: scripts/gofmt.sh <write|check> <root|sdk>\n' >&2
    exit 2
    ;;
esac

case "$scope" in
  root)
    set -- .
    exclude_sdk=true
    ;;
  sdk)
    set -- tests/compat/openai-sdk/go
    exclude_sdk=false
    ;;
  *)
    printf 'usage: scripts/gofmt.sh <write|check> <root|sdk>\n' >&2
    exit 2
    ;;
esac

if [ "$exclude_sdk" = true ]; then
  if [ "$mode" = write ]; then
    find "$@" \
      \( -path './.git' -o -path './node_modules' -o -path '*/node_modules' -o -path './tests/compat/openai-sdk/go' \) -prune \
      -o -type f -name '*.go' -exec gofmt -w {} +
    exit 0
  fi
  output=$(find "$@" \
    \( -path './.git' -o -path './node_modules' -o -path '*/node_modules' -o -path './tests/compat/openai-sdk/go' \) -prune \
    -o -type f -name '*.go' -exec gofmt -l {} +)
else
  if [ "$mode" = write ]; then
    find "$@" -type f -name '*.go' -exec gofmt -w {} +
    exit 0
  fi
  output=$(find "$@" -type f -name '*.go' -exec gofmt -l {} +)
fi

if [ -n "$output" ]; then
  printf '%s\n' "$output"
  exit 1
fi
