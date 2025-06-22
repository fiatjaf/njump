#!/usr/bin/env bash

set -e

# Check for goi18n
if ! command -v /home/lio/go/bin/goi18n >/dev/null 2>&1; then
  echo "goi18n not found. Install it with: go install github.com/nicksnyder/go-i18n/v2/goi18n@latest" >&2
  exit 1
fi

# Extract and merge translation strings
# This updates active.en.json and merges strings into other locale files

/home/lio/go/bin/goi18n extract -format json -outdir locales

/home/lio/go/bin/goi18n merge -format json -outdir locales locales/*.json locales/*.toml

