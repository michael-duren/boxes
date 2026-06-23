#!/usr/bin/env bash

set -eou pipefail

ctr="boxes-ctr"

echo "checking deps"

if ! command -v docker &>/dev/null; then
    echo "docker is not installed exiting"
    exit 1
fi

echo "building $ctr"
docker build -t "$ctr" .
docker run -it "$ctr"
