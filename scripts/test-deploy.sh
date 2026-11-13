#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
tmp_dir=$(mktemp -d)
trap 'rm -f "$tmp_dir/kubectl" "$tmp_dir/docker" "$tmp_dir/calls"; rmdir "$tmp_dir"' EXIT

cat > "$tmp_dir/kubectl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$CALLS"
if [[ "$*" == "config current-context" ]]; then
    echo test-context
fi
EOF
cat > "$tmp_dir/docker" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp_dir/kubectl" "$tmp_dir/docker"
export PATH="$tmp_dir:$PATH" CALLS="$tmp_dir/calls"
export AZURE_SUBSCRIPTION_ID=test-sub TM_RESOURCE_GROUP=test-tm DNS_ZONE_RESOURCE_GROUP=test-dns
export AZURE_TENANT_ID=test-tenant AZURE_CLIENT_ID=test-client

printf '\n\n\n\n\nregistry.example\nexample.com\ny\nn\n' | bash deploy/deploy.sh >/dev/null

rbac=$(grep -n 'apply -f .*/examples/single-cluster/rbac.yaml' "$CALLS" | cut -d: -f1)
identity=$(grep -n 'annotate serviceaccount external-dns -n external-dns azure.workload.identity/client-id=test-client --overwrite' "$CALLS" | cut -d: -f1)
crd=$(grep -n 'apply -f .*/deploy/kubernetes/dnsendpoint-crd.yaml' "$CALLS" | cut -d: -f1)
wait_crd=$(grep -n 'wait --for=condition=Established --timeout=60s crd/dnsendpoints.externaldns.k8s.io' "$CALLS" | cut -d: -f1)
deployment=$(grep -n 'apply -f /.*' "$CALLS" | tail -n 1 | cut -d: -f1)
test "$rbac" -lt "$identity" && test "$identity" -lt "$crd" &&
    test "$crd" -lt "$wait_crd" && test "$wait_crd" -lt "$deployment"
