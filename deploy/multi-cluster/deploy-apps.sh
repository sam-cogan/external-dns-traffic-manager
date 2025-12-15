#!/bin/bash
set -e

# Load configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"

echo "=========================================="
echo "Deploying Demo Applications"
echo "=========================================="
echo ""

# Function to deploy app to a cluster
deploy_app() {
    local CLUSTER_NAME=$1
    local CLUSTER_CONTEXT=$2
    local REGION=$3
    local REGION_NAME=$4
    local APP_HOSTNAME_REGION=$5
    local PRIORITY=$6
    
    echo "Deploying application to ${CLUSTER_NAME} (${REGION})..."
    kubectl config use-context "${CLUSTER_CONTEXT}"
    
    cat <<EOF | kubectl apply -f -
---
# Demo Application - ${REGION_NAME}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${APP_NAME}
  namespace: default
  labels:
    app: ${APP_NAME}
    region: ${REGION}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ${APP_NAME}
      region: ${REGION}
  template:
    metadata:
      labels:
        app: ${APP_NAME}
        region: ${REGION}
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 100m
            memory: 128Mi
        volumeMounts:
        - name: content
          mountPath: /usr/share/nginx/html
      volumes:
      - name: content
        configMap:
          name: ${APP_NAME}-content
---
# ConfigMap with custom content
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${APP_NAME}-content
  namespace: default
data:
  index.html: |
    <!DOCTYPE html>
    <html>
    <head>
        <title>Multi-Cluster Demo - ${REGION_NAME}</title>
        <style>
            body {
                font-family: Arial, sans-serif;
                display: flex;
                justify-content: center;
                align-items: center;
                height: 100vh;
                margin: 0;
                background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
                color: white;
            }
            .container {
                text-align: center;
                padding: 40px;
                background: rgba(0,0,0,0.3);
                border-radius: 10px;
            }
            h1 { font-size: 48px; margin: 20px 0; }
            p { font-size: 24px; }
            .region { color: #00ff00; font-weight: bold; }
            .cluster { color: #ffff00; font-weight: bold; }
        </style>
    </head>
    <body>
        <div class="container">
            <h1>🌍 Multi-Cluster Traffic Manager Demo</h1>
            <p>Region: <span class="region">${REGION_NAME}</span></p>
            <p>Cluster: <span class="cluster">${CLUSTER_NAME}</span></p>
            <p>This service is load-balanced across multiple AKS clusters</p>
            <p>Traffic Manager Priority: ${PRIORITY}</p>
        </div>
    </body>
    </html>
---
# Service with Traffic Manager annotations
apiVersion: v1
kind: Service
metadata:
  name: ${APP_NAME}
  namespace: default
  annotations:
    # DNS name for this specific endpoint (Azure DNS creates A record)
    external-dns.alpha.kubernetes.io/hostname: ${APP_HOSTNAME_REGION}
    
    # Traffic Manager configuration for vanity URL
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-enabled: "true"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-hostname: "${APP_HOSTNAME}"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-resource-group: "${TM_PROFILE_RG}"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-routing-method: "Weighted"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-weight: "100"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-priority: "${PRIORITY}"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-endpoint-name: "${APP_NAME}-${REGION}"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-endpoint-location: "${REGION_NAME}"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-health-checks-enabled: "true"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-monitor-protocol: "HTTP"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-monitor-port: "80"
    external-dns.alpha.kubernetes.io/webhook-traffic-manager-monitor-path: "/"
spec:
  type: LoadBalancer
  selector:
    app: ${APP_NAME}
    region: ${REGION}
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80
EOF
    
    echo "✓ Application deployed to ${CLUSTER_NAME}"
    echo ""
}

# Deploy to both clusters
deploy_app "${CLUSTER_EAST_NAME}" "aks-tm-east" "east" "East US" "${APP_EAST_HOSTNAME}" "1"
deploy_app "${CLUSTER_WEST_NAME}" "aks-tm-west" "west" "West US" "${APP_WEST_HOSTNAME}" "2"

echo "=========================================="
echo "✓ Application Deployment Complete"
echo "=========================================="
echo ""
echo "Waiting for LoadBalancer IPs to be assigned..."
echo ""

# Wait for LoadBalancer IPs
echo "East cluster:"
kubectl --context aks-tm-east get service ${APP_NAME} -n default -w &
WAIT_PID_EAST=$!
sleep 2

echo ""
echo "West cluster:"
kubectl --context aks-tm-west get service ${APP_NAME} -n default -w &
WAIT_PID_WEST=$!

# Wait up to 5 minutes
sleep 60
kill $WAIT_PID_EAST $WAIT_PID_WEST 2>/dev/null || true

echo ""
echo "Next step:"
echo "  ./verify.sh    # Verify Traffic Manager configuration"
