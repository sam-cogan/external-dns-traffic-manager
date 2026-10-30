#!/usr/bin/env bash
# Offline fixtures for the real deployment scripts; never used for deployment.
set -euo pipefail
exec 3>&1 1>&2
SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000
TM_PROFILE_RG=test-tm
DNS_ZONE_RG=test-dns
DNS_ZONE_NAME=example.com
IMAGE_REGISTRY=example.invalid
IMAGE_NAME=traffic-manager-webhook
IMAGE_TAG=ci
CLUSTER_EAST_NAME=test-east
CLUSTER_WEST_NAME=test-west
CLUSTER_EAST_REGION=eastus
CLUSTER_WEST_REGION=westus
APP_NAME=demo
APP_HOSTNAME=demo.example.com
APP_EAST_HOSTNAME=east.example.com
APP_WEST_HOSTNAME=west.example.com

az() {
    if [[ "$*" != "account show --query tenantId -o tsv" ]]; then
        echo "Unsupported offline Azure command: $*" >&2
        return 1
    fi
    printf '%s\n' 00000000-0000-0000-0000-000000000000
}

kubectl() {
    case "$1 $2" in
        "config use-context"|"delete deployment"|"wait --for=delete"|"wait --for=condition=available")
            return 0 ;;
        "apply -f")
            if [[ "$3" == "-" ]]; then
                cat >&3
            else
                cat "$3" >&3
            fi
            printf '\n---\n' >&3 ;;
        "create namespace"|"create secret"|"create configmap")
            python3 - "$@" <<'PY'
import json
import sys

args = sys.argv[1:]
resource = args[1]
kind = {"namespace": "Namespace", "secret": "Secret", "configmap": "ConfigMap"}[resource]
name = args[3] if resource == "secret" else args[2]
metadata = {"name": name}
if "--namespace" in args:
    metadata["namespace"] = args[args.index("--namespace") + 1]
obj = {"apiVersion": "v1", "kind": kind, "metadata": metadata}
data = {}
for arg in args:
    if arg.startswith("--from-literal="):
        key, value = arg.removeprefix("--from-literal=").split("=", 1)
        data[key] = value
if data:
    obj["stringData" if resource == "secret" else "data"] = data
print(json.dumps(obj))
PY
            ;;
        *)
            echo "Unsupported offline kubectl command: $*" >&2
            return 1 ;;
    esac
}
