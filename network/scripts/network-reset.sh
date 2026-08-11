#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

"${SCRIPT_DIR}/network-down.sh"

echo "Removing generated crypto material..."
rm -rf "${ORGANIZATIONS_DIR:?}/"

echo "Removing channel artifacts..."
rm -rf "${CHANNEL_ARTIFACTS_DIR:?}/"

echo "Network reset complete."
