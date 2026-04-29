#!/usr/bin/env bash
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

# -------------------------------------------------------------------
# setup-site-agent-core-grpc-certs.sh
#
# One-shot helper to provision TLS material for carbide-rest-site-agent
# gRPC client connections to NCX Infra Controller Core:
#
#   1. Ensure namespace exists
#   2. Ensure ca-signing-secret (carbide-rest + cert-manager) — runs gen-site-ca.sh if missing
#   3. Apply cert-manager.io ClusterIssuer carbide-rest-ca-issuer
#   4. Apply Certificate core-grpc-client-site-agent-certs (→ Secret mounted at /etc/carbide)
#   5. Wait for Certificate Ready + verify Secret
#   6. Optionally rollout restart carbide-rest-site-agent
#
# Prerequisites:
#   - kubectl configured for your cluster
#   - cert-manager installed (CRD certificates.cert-manager.io)
#
# Usage:
#   ./scripts/setup-site-agent-core-grpc-certs.sh
#   ./scripts/setup-site-agent-core-grpc-certs.sh --namespace carbide-rest
#   ./scripts/setup-site-agent-core-grpc-certs.sh --no-restart
#   ./scripts/setup-site-agent-core-grpc-certs.sh --skip-gen-ca   # fail if CA secret missing
#
# Forge / custom SPIFFE:
#   If Core expects a different URI than the default in certificate.yaml, edit that file
#   or the Helm chart values before running; this script applies the repo manifest as-is.
#
# See also: deploy/INSTALLATION.md (Steps 2, 6, 13), scripts/gen-site-ca.sh
# -------------------------------------------------------------------

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }
ok()   { echo "OK: $*"; }

NAMESPACE="${NAMESPACE:-carbide-rest}"
CERT_WAIT_TIMEOUT="${CERT_WAIT_TIMEOUT:-300s}"
RESTART_AGENT=true
SKIP_GEN_CA=false

usage() {
  cat <<'EOF'
Usage: scripts/setup-site-agent-core-grpc-certs.sh [options]

Ensures CA secrets, ClusterIssuer carbide-rest-ca-issuer, and Certificate
core-grpc-client-site-agent-certs (Secret for site-agent gRPC to Core).

Options:
  --namespace <ns>   Carbide namespace (default: carbide-rest, or env NAMESPACE)
  --timeout <dur>    kubectl wait timeout for Certificate (default: 300s)
  --no-restart       Do not rollout restart carbide-rest-site-agent
  --skip-gen-ca      Fail if ca-signing-secret is missing instead of running gen-site-ca.sh
  -h, --help         Show this help

Prerequisites: kubectl, cluster access, cert-manager in namespace cert-manager.

Examples:
  ./scripts/setup-site-agent-core-grpc-certs.sh
  NAMESPACE=my-ns ./scripts/setup-site-agent-core-grpc-certs.sh
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)     NAMESPACE="${2:?}"; shift 2 ;;
    --timeout)       CERT_WAIT_TIMEOUT="${2:?}"; shift 2 ;;
    --no-restart)    RESTART_AGENT=false; shift ;;
    --skip-gen-ca)   SKIP_GEN_CA=true; shift ;;
    -h|--help)       usage ;;
    *) die "Unknown option: $1 (use --help)" ;;
  esac
done

cd "$REPO_ROOT"

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }

check_prereqs() {
  need_cmd kubectl
  kubectl cluster-info >/dev/null 2>&1 || die "kubectl cannot reach the cluster"
  kubectl get crd certificates.cert-manager.io >/dev/null 2>&1 || \
    die "cert-manager CRD certificates.cert-manager.io not found. Install cert-manager first."
  kubectl get deployment -n cert-manager cert-manager >/dev/null 2>&1 || \
    die "cert-manager deployment not found in namespace cert-manager. Install cert-manager first."
}

ensure_namespace() {
  if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    info "Creating namespace $NAMESPACE"
    kubectl create namespace "$NAMESPACE"
  fi
}

ca_present() {
  kubectl get secret ca-signing-secret -n "$NAMESPACE" >/dev/null 2>&1 \
    && kubectl get secret ca-signing-secret -n cert-manager >/dev/null 2>&1
}

ensure_ca() {
  if ca_present; then
    ok "ca-signing-secret exists in $NAMESPACE and cert-manager"
    return 0
  fi
  if [[ "$SKIP_GEN_CA" == true ]]; then
    die "ca-signing-secret missing in $NAMESPACE or cert-manager. Run scripts/gen-site-ca.sh or omit --skip-gen-ca"
  fi
  info "Generating / installing CA (scripts/gen-site-ca.sh --namespace $NAMESPACE)"
  bash "$REPO_ROOT/scripts/gen-site-ca.sh" --namespace "$NAMESPACE"
  ca_present || die "gen-site-ca.sh did not leave ca-signing-secret in both namespaces"
  ok "CA secrets installed"
}

apply_cluster_issuer() {
  info "Applying ClusterIssuer carbide-rest-ca-issuer"
  kubectl apply -k "$REPO_ROOT/deploy/kustomize/base/cert-manager-io"
  # ClusterIssuer becomes Ready asynchronously; poll status for a short window
  local i=0
  while [[ $i -lt 60 ]]; do
    if kubectl get clusterissuer carbide-rest-ca-issuer -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q True; then
      ok "ClusterIssuer carbide-rest-ca-issuer is Ready"
      return 0
    fi
    sleep 2
    i=$((i + 1))
  done
  warn_out="ClusterIssuer carbide-rest-ca-issuer not Ready yet; cert issuance may still succeed once CA propagates."
  echo "WARN: $warn_out" >&2
}

apply_site_agent_certificate() {
  info "Applying Certificate core-grpc-client-site-agent-certs in $NAMESPACE"
  kubectl apply -n "$NAMESPACE" -f "$REPO_ROOT/deploy/kustomize/base/site-agent/certificate.yaml"
}

wait_certificate_ready() {
  info "Waiting up to $CERT_WAIT_TIMEOUT for Certificate to become Ready"
  if kubectl wait --for=condition=Ready "certificate/core-grpc-client-site-agent-certs" \
      -n "$NAMESPACE" --timeout="$CERT_WAIT_TIMEOUT" 2>/dev/null; then
    ok "Certificate core-grpc-client-site-agent-certs is Ready"
    return 0
  fi
  echo "--- Certificate status ---" >&2
  kubectl describe certificate core-grpc-client-site-agent-certs -n "$NAMESPACE" >&2 || true
  die "Certificate did not become Ready within $CERT_WAIT_TIMEOUT. See cert-manager logs / describe output above."
}

verify_tls_secret() {
  kubectl get secret core-grpc-client-site-agent-certs -n "$NAMESPACE" >/dev/null 2>&1 || \
    die "Secret core-grpc-client-site-agent-certs not found"
  ok "Secret core-grpc-client-site-agent-certs exists (mounted by site-agent at /etc/carbide)"
  kubectl get secret core-grpc-client-site-agent-certs -n "$NAMESPACE" -o wide
}

maybe_restart_site_agent() {
  if [[ "$RESTART_AGENT" != true ]]; then
    info "Skipping site-agent restart (--no-restart)"
    return 0
  fi
  if ! kubectl get statefulset carbide-rest-site-agent -n "$NAMESPACE" >/dev/null 2>&1; then
    echo "WARN: StatefulSet carbide-rest-site-agent not found; skip rollout restart." >&2
    return 0
  fi
  info "Restarting carbide-rest-site-agent to pick up renewed certs (if any)"
  kubectl rollout restart statefulset/carbide-rest-site-agent -n "$NAMESPACE"
  kubectl rollout status statefulset/carbide-rest-site-agent -n "$NAMESPACE" --timeout=240s || true
  ok "Site agent rollout triggered"
}

main() {
  check_prereqs
  ensure_namespace
  ensure_ca
  apply_cluster_issuer
  apply_site_agent_certificate
  wait_certificate_ready
  verify_tls_secret
  maybe_restart_site_agent
  echo ""
  ok "Done. gRPC client cert for Core: secret/$NAMESPACE/core-grpc-client-site-agent-certs"
  echo "    Mount path in site-agent: /etc/carbide (cert-manager keys: ca.crt, tls.crt, tls.key — same as grpcurl -cacert/-cert/-key)"
  echo "    Forge carbide-api: CARBIDE_ADDRESS + CARBIDE_SEC_OPT=2 (Helm). If TLS fails with unknown authority,"
  echo "    the client secret ca.crt may not sign the API server cert — copy forge CA into a Secret and set"
  echo "    CARBIDE_TLS_VERIFY_CA_PATH or Helm secrets.carbideApiServerCA (see helm/charts/carbide-rest-site-agent/values.yaml)."
}

main "$@"
