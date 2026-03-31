#!/bin/sh

set -eu

git config core.hooksPath .githooks
printf '%s\n' 'configured git hooks path: .githooks'
