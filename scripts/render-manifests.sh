#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .ci/rendered
bash deploy/multi-cluster/deploy-webhooks.sh --render > .ci/rendered/webhooks.yaml
bash deploy/multi-cluster/deploy-apps.sh --render > .ci/rendered/apps.yaml
