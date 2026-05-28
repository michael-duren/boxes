#!/usr/bin/env bash
# assumed you are running from repo root

alpine_dir="alpinefs"
fs_dir="rootfs"
path="$alpine_dir/$fs_dir"

# exit, unset vars, o pipefail
set -eou pipefail

if [[ ! -e "$path" ]]; then
    echo "creating $alpine_dir/$fs_dir in repo root"
    mkdir -p "$path"
else
    echo "removing previous installation at path: $path"
    read -rp "are you sure you want to do this? (Y/n)" answer
    if [[ "${answer,,}" != "y" ]]; then
        echo "user selected no, exiting script"
        exit 0
    fi
fi

if ! command -v docker &>/dev/null || command -v runc &>/dev/null; then
    echo "missing Docker, please install"
    exit 1
fi

cd "$alpine_dir"

# create alpinefs locally
docker export $(docker create alpine) | tar -C "$fs_dir" -xvf -
echo "created alpinefs"

runc spec
echo "created config with runc"
