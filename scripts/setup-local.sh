#!/bin/bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

NAMESPACE="${NAMESPACE:-carbide-rest}"
API_URL="${API_URL:-http://localhost:8388}"
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8082}"
ORG="${ORG:-test-org}"

usage() {
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  pki         Setup PKI secrets and CA"
    echo "  site-agent  Setup site-agent with a real site"
    echo "  all         Run both pki and site-agent setup"
    echo "  verify      Verify local deployment health"
    exit 1
}

# ============================================================================
# PKI Setup Functions
# ============================================================================

generate_ca() {
    echo "Generating CA certificate..."

    CA_DIR=$(mktemp -d)
    trap "rm -rf $CA_DIR" RETURN

    cat > "$CA_DIR/ca.cnf" << 'EOFCNF'
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_ca
prompt = no

[req_distinguished_name]
C = US
ST = CA
L = Local
O = Carbide Dev
OU = Dev
CN = carbide-local-ca

[v3_ca]
basicConstraints = critical,CA:TRUE
keyUsage = critical,keyCertSign,cRLSign,digitalSignature
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer:always
EOFCNF

    openssl req -x509 -sha256 -nodes -newkey rsa:4096 \
        -keyout "$CA_DIR/ca.key" \
        -out "$CA_DIR/ca.crt" \
        -days 3650 \
        -config "$CA_DIR/ca.cnf" \
        -extensions v3_ca

    # Create CA secret in carbide-rest namespace for credsmgr
    kubectl create secret tls ca-signing-secret \
        --cert="$CA_DIR/ca.crt" \
        --key="$CA_DIR/ca.key" \
        -n "$NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f -

    # Create CA secret in cert-manager namespace for ClusterIssuer
    kubectl create secret tls ca-signing-secret \
        --cert="$CA_DIR/ca.crt" \
        --key="$CA_DIR/ca.key" \
        -n cert-manager \
        --dry-run=client -o yaml | kubectl apply -f -

    echo "CA secret created in both namespaces"
}

create_service_certs() {
    echo "Creating service secrets..."

    # Note: core-grpc-client-site-agent-certs and site-manager-tls are now managed by
    # cert-manager.io Certificate resources (see deploy/kustomize/base/site-agent/certificate.yaml
    # and deploy/kustomize/base/site-manager/certificate.yaml). They will be issued
    # automatically once the carbide-rest-ca-issuer ClusterIssuer is applied.

    CA_CERT=$(kubectl get secret ca-signing-secret -n "$NAMESPACE" -o jsonpath='{.data.tls\.crt}' | base64 -d 2>/dev/null || echo "")

    if [ -z "$CA_CERT" ]; then
        echo "Warning: Could not retrieve CA certificate"
        return 0
    fi

    kubectl create secret generic site-registration \
        --from-literal=site-uuid="00000000-0000-4000-8000-000000000001" \
        --from-literal=otp="local-dev-otp-token" \
        --from-literal=creds-url="http://carbide-rest-site-manager:8100/v1/site/credentials" \
        --from-literal=cacert="$CA_CERT" \
        -n "$NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f -

    echo "Service secrets created"
}

setup_pki() {
    echo "Setting up local PKI..."
    kubectl get ns "$NAMESPACE" > /dev/null 2>&1 || kubectl create ns "$NAMESPACE"
    generate_ca
    create_service_certs
    echo "PKI setup complete."
}

# ============================================================================
# Site-Agent Setup Functions
# ============================================================================

wait_for_services() {
    echo "Waiting for Keycloak..."
    for i in {1..240}; do
        if curl -sf "$KEYCLOAK_URL/realms/carbide-dev" > /dev/null 2>&1; then
            break
        fi
        if [ $i -eq 240 ]; then
            echo "ERROR: Keycloak not ready"
            exit 1
        fi
        sleep 1
        echo "Waiting for Keycloak... $i/240"
    done

    # Once Keycloak is ready we need to restart the API server because if Keycloak wasn't ready
    # when it started it would have failed to fetch the JWKS, and therefore it will automatically
    # disable Keycloak support.
    echo "Waiting for API ..."
    kubectl -n $NAMESPACE rollout restart deployment carbide-rest-api
    if ! kubectl -n $NAMESPACE rollout status deployment carbide-rest-api --timeout=240s; then
        echo "ERROR: Failed to restart API"
        exit 1
    fi

    echo "Waiting for site-manager..."
    if ! kubectl -n $NAMESPACE wait --for=condition=ready pod -l app=carbide-rest-site-manager --timeout=360s; then
        echo "ERROR: Site-manager not ready"
        kubectl -n $NAMESPACE get pods -l app=carbide-rest-site-manager
        exit 1
    fi
}

get_token() {
    TOKEN=$(curl -sf -X POST "$KEYCLOAK_URL/realms/carbide-dev/protocol/openid-connect/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "client_id=carbide-api" \
        -d "client_secret=carbide-local-secret" \
        -d "grant_type=password" \
        -d "username=admin@example.com" \
        -d "password=adminpassword" | jq -r .access_token)

    if [ -z "$TOKEN" ] || [ "$TOKEN" == "null" ]; then
        echo "ERROR: Failed to acquire token"
        exit 1
    fi
    echo "$TOKEN"
}

create_site() {
    local token=$1

    curl -sf "$API_URL/v2/org/$ORG/carbide/tenant/current" \
        -H "Authorization: Bearer $token" > /dev/null 2>&1 || true

    PROVIDER_RESP=$(curl -sf "$API_URL/v2/org/$ORG/carbide/infrastructure-provider/current" \
        -H "Authorization: Bearer $token" 2>/dev/null || echo "{}")

    PROVIDER_ID=$(echo "$PROVIDER_RESP" | jq -r '.id // empty')
    if [ -z "$PROVIDER_ID" ]; then
        PROVIDER_RESP=$(curl -sf -X POST "$API_URL/v2/org/$ORG/carbide/infrastructure-provider" \
            -H "Authorization: Bearer $token" \
            -H "Content-Type: application/json" \
            -d '{"name": "Local Dev Provider", "description": "Local development infrastructure provider"}')
        PROVIDER_ID=$(echo "$PROVIDER_RESP" | jq -r '.id')
    fi

    EXISTING_SITE=$(curl -sf "$API_URL/v2/org/$ORG/carbide/site?infrastructureProviderId=$PROVIDER_ID" \
        -H "Authorization: Bearer $token" | jq -r '.[] | select(.name == "local-dev-site") | .id' 2>/dev/null || echo "")

    if [ -n "$EXISTING_SITE" ] && [ "$EXISTING_SITE" != "null" ]; then
        echo "$EXISTING_SITE"
        return
    fi

    for attempt in 1 2 3; do
        FULL=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/v2/org/$ORG/carbide/site?infrastructureProviderId=$PROVIDER_ID" \
            -H "Authorization: Bearer $token" \
            -H "Content-Type: application/json" \
            -d '{
                "name": "local-dev-site",
                "description": "Local development site",
                "location": {"address": "Local Development", "city": "Santa Clara", "state": "CA", "country": "USA", "postalCode": "95054"},
                "contact": {"name": "Dev Team", "email": "dev@example.com", "phone": "555-0100"}
            }')
        HTTP_CODE=$(echo "$FULL" | tail -n 1)
        SITE_RESP=$(echo "$FULL" | sed '$d')

        SITE_ID=$(echo "$SITE_RESP" | jq -r '.id // empty')
        if [ -n "$SITE_ID" ] && [ "$SITE_ID" != "null" ]; then
            echo "$SITE_ID"
            return
        fi

        if [ $attempt -lt 3 ]; then
            echo "Site creation attempt $attempt failed (HTTP $HTTP_CODE), retrying..." >&2
            echo "Response: $SITE_RESP" >&2
            read -t 5 < /dev/null || true
        fi
    done

    echo "ERROR: Failed to create site (HTTP $HTTP_CODE)" >&2
    echo "Response: $SITE_RESP" >&2
    exit 1
}

configure_site_agent() {
    local site_id=$1

    kubectl -n $NAMESPACE run temporal-ns-create-$site_id --rm -it --restart=Never \
        --image=temporalio/admin-tools:1.26.2 \
        --overrides='{"spec":{"containers":[{"name":"temporal-ns-create","image":"temporalio/admin-tools:1.26.2","command":["temporal","operator","namespace","create","'"$site_id"'","--address","temporal-frontend.temporal:7233","--tls","--tls-disable-host-verification","--tls-ca-path","/etc/temporal/certs/ca.crt"],"volumeMounts":[{"name":"tls-certs","mountPath":"/etc/temporal/certs","readOnly":true}]}],"volumes":[{"name":"tls-certs","secret":{"secretName":"workflow-temporal-client-tls"}}]}}' \
        2>/dev/null || true

    kubectl -n $NAMESPACE get configmap carbide-rest-site-agent-config -o yaml | \
        sed "s/CLUSTER_ID: .*/CLUSTER_ID: \"$site_id\"/" | \
        sed "s/TEMPORAL_SUBSCRIBE_NAMESPACE: .*/TEMPORAL_SUBSCRIBE_NAMESPACE: \"$site_id\"/" | \
        sed "s/TEMPORAL_SUBSCRIBE_QUEUE: .*/TEMPORAL_SUBSCRIBE_QUEUE: \"$site_id\"/" | \
        kubectl apply -f -

    kubectl -n $NAMESPACE get secret site-registration -o yaml 2>/dev/null | \
        sed "s/site-uuid: .*/site-uuid: $(echo -n $site_id | base64)/" | \
        kubectl apply -f - 2>/dev/null || \
        kubectl -n $NAMESPACE create secret generic site-registration \
            --from-literal=site-uuid="$site_id" \
            --from-literal=otp="local-dev-otp" \
            --from-literal=creds-url="http://carbide-rest-site-manager:8100/v1/site/credentials" \
            --from-literal=cacert=""

    kubectl -n $NAMESPACE rollout restart deployment/carbide-rest-site-agent
    kubectl -n $NAMESPACE rollout status deployment/carbide-rest-site-agent --timeout=240s
}

setup_site_agent() {
    echo "Setting up site-agent..."
    wait_for_services

    echo "Allowing API and Temporal to stabilize..."
    sleep 10

    echo "Acquiring token..."
    TOKEN=$(get_token)

    echo "Creating site..."
    SITE_ID=$(create_site "$TOKEN")
    echo "Site ID: $SITE_ID"

    echo "Configuring site-agent..."
    configure_site_agent "$SITE_ID"

    kubectl -n $NAMESPACE get pods -l app=carbide-rest-site-agent
    echo "Site-agent setup complete."
}

# ============================================================================
# Verify Functions
# ============================================================================

verify() {
    echo "Verifying local deployment..."

    echo -n "API health... "
    if curl -sf "$API_URL/healthz" 2>/dev/null | jq -e '.is_healthy == true' > /dev/null 2>&1; then
        echo "[OK]"
    else
        echo "[FAIL]"
    fi

    echo -n "Keycloak realm... "
    if curl -sf "$KEYCLOAK_URL/realms/carbide-dev" 2>/dev/null | jq -e '.realm == "carbide-dev"' > /dev/null 2>&1; then
        echo "[OK]"
    else
        echo "[FAIL]"
    fi

    echo -n "Cert manager... "
    if kubectl -n "$NAMESPACE" get deployment carbide-rest-cert-manager -o jsonpath='{.status.readyReplicas}' 2>/dev/null | grep -q "[1-9]"; then
        echo "[OK]"
    else
        echo "[WARN]"
    fi

    echo ""
    kubectl -n $NAMESPACE get pods 2>/dev/null || echo "Could not get pod status"
}

# ============================================================================
# Main
# ============================================================================

case "${1:-}" in
    pki)
        setup_pki
        ;;
    site-agent)
        setup_site_agent
        ;;
    all)
        setup_pki
        setup_site_agent
        ;;
    verify)
        verify
        ;;
    *)
        usage
        ;;
esac
