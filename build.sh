#!/usr/bin/env sh
set -eu
mkdir -p dist
case "$(uname -s)" in
  Darwin) out="dist/xai-403-fixer.dylib" ;;
  *) out="dist/xai-403-fixer.so" ;;
esac
CGO_ENABLED=1 go build -buildmode=c-shared -trimpath -ldflags='-s -w' -o "$out" .
