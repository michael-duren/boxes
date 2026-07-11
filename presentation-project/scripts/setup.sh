#!/usr/bin/env bash
#
# Build the "evil node" image and export its filesystem into ./rootfs so the
# box runtime can run it. The image (scripts/Dockerfile) already swaps the real
# node for the escape probe, so there is nothing to patch in the tree here --
# we just build and export.
#
# Run from the presentation-project/ root:   bash scripts/setup.sh
#
set -euo pipefail

C_GRN=$'\033[32m'
C_CYN=$'\033[36m'
C_YEL=$'\033[33m'
C_RST=$'\033[0m'

IMAGE=${IMAGE:-evilnode}
ROOTFS=${ROOTFS:-rootfs}
BASE=${BASE:-node:latest}

# Build context is the project root (Dockerfile needs scripts/ + webapp/).
# .dockerignore keeps the exported rootfs out of the context.
echo -e "${C_CYN}> building ${IMAGE} image (base: ${BASE})${C_RST}"
docker build --build-arg BASE="${BASE}" -t "${IMAGE}" .

echo -e "${C_CYN}> exporting image filesystem -> ./${ROOTFS}/${C_RST}"
rm -rf "${ROOTFS}"
mkdir -p "${ROOTFS}"

cid="$(docker create "${IMAGE}")"
trap 'docker rm -f "${cid}" >/dev/null 2>&1 || true' EXIT
docker export "${cid}" | tar -C "${ROOTFS}" -xf -

echo -e "${C_GRN}> done: ./${ROOTFS} ready${C_RST}"
echo -e "${C_YEL}  node       -> escape probe (harmless, read-only)"
echo -e "  node.real  -> the genuine interpreter"
echo -e "  probe log  -> ${ROOTFS}/var/log/evilnode.log (after it runs)${C_RST}"
