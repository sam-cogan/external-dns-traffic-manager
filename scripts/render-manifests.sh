#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .ci/rendered
bash deploy/multi-cluster/deploy-webhooks.sh --render > .ci/rendered/webhooks.yaml
bash deploy/multi-cluster/deploy-apps.sh --render > .ci/rendered/apps.yaml
SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000 \
TM_RG=test-tm \
DNS_RG=test-dns \
DOMAIN_FILTER=example.com \
TENANT_ID=00000000-0000-0000-0000-000000000000 \
CLIENT_ID=00000000-0000-0000-0000-000000000000 \
FULL_IMAGE=example.invalid/traffic-manager-webhook:ci \
    bash deploy/deploy.sh --render > .ci/rendered/single-cluster.yaml
if grep -qE '__[A-Z_]+__' .ci/rendered/single-cluster.yaml; then
    echo "single-cluster deployment manifest has unresolved substitution tokens" >&2
    exit 1
fi
