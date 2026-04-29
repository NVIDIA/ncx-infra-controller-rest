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

# verify-site-agent-carbide-grpc.sh — Inspect site-agent Core gRPC config and probe carbide-api.
#
# Run from your machine (needs kubectl + cluster access). This repo cannot reach your cluster.
#
# Usage:
#   ./scripts/verify-site-agent-carbide-grpc.sh
#   NAMESPACE=carbide-rest ./scripts/verify-site-agent-carbide-grpc.sh
#   ./scripts/verify-site-agent-carbide-grpc.sh --namespace carbide-rest --run-namespace default
#
# Plaintext gRPC (Forge :1079): set CARBIDE_SEC_OPT: "0" in carbide-rest-site-agent-config.
# mTLS client cert rotation (cert-manager): ./scripts/setup-site-agent-core-grpc-certs.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }
warn() { echo "WARN: $*" >&2; }

NAMESPACE="${NAMESPACE:-carbide-rest}"
# Run the grpcurl test pod in the same namespace as site-agent by default (avoids NS network isolation).
RUN_NS="${RUN_NS:-$NAMESPACE}"
DO_GRPCURL=true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)     NAMESPACE="${2:?}"; shift 2 ;;
    --run-namespace) RUN_NS="${2:?}"; shift 2 ;;
    --no-grpcurl)    DO_GRPCURL=false; shift ;;
    -h|--help)
      head -n 26 "$0" | tail -n +2
      exit 0
      ;;
    *) die "unknown option: $1 (try --help)" ;;
  esac
done

need() { command -v "$1" >/dev/null || die "missing command: $1"; }
need kubectl

kubectl cluster-info >/dev/null 2>&1 || die "kubectl cannot reach the cluster"

info "Site-agent namespace: $NAMESPACE | ephemeral grpcurl pod namespace: $RUN_NS"

kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || die "namespace $NAMESPACE not found"
kubectl get namespace "$RUN_NS" >/dev/null 2>&1 || die "namespace $RUN_NS not found"

CM=carbide-rest-site-agent-config
kubectl get configmap "$CM" -n "$NAMESPACE" >/dev/null 2>&1 || die "ConfigMap $CM not found in $NAMESPACE"

ADDR=$(kubectl get configmap "$CM" -n "$NAMESPACE" -o jsonpath='{.data.CARBIDE_ADDRESS}' 2>/dev/null || true)
SEC=$(kubectl get configmap "$CM" -n "$NAMESPACE" -o jsonpath='{.data.CARBIDE_SEC_OPT}' 2>/dev/null || true)

echo ""
echo "--- $CM (Core gRPC) ---"
echo "CARBIDE_ADDRESS=${ADDR:-<empty>}"
echo "CARBIDE_SEC_OPT=${SEC:-<empty>}"
echo ""

if [[ -z "${ADDR// }" ]]; then
  warn "CARBIDE_ADDRESS empty → site-agent uses code default (config_manager.go), typically carbide-api.forge-system.svc.cluster.local:1079"
fi

if [[ -z "${SEC// }" ]]; then
  warn "CARBIDE_SEC_OPT empty → site-agent defaults to ServerTLS (1) on parse error; use explicit \"0\" for plaintext gRPC to Forge."
fi

POD=$(kubectl get pods -n "$NAMESPACE" -l app=carbide-rest-site-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [[ -n "$POD" ]]; then
  info "Site agent pod: $POD"
  kubectl exec -n "$NAMESPACE" "$POD" -- printenv CARBIDE_ADDRESS CARBIDE_SEC_OPT 2>/dev/null || warn "exec into site-agent failed"
else
  warn "No pod with label app=carbide-rest-site-agent in $NAMESPACE"
fi

echo ""
info "Recent site-agent logs (Carbide / GRPC / error):"
kubectl logs -n "$NAMESPACE" -l app=carbide-rest-site-agent --tail=40 2>/dev/null | grep -iE 'carbide|grpc|forge|error|fail' || echo "(no matching lines in last 40 log lines)"
echo ""

if [[ "$DO_GRPCURL" != true ]]; then
  info "Skipping grpcurl (--no-grpcurl)"
  exit 0
fi

TARGET="${ADDR:-carbide-api.forge-system.svc.cluster.local:1079}"
PLAINTEXT_ARGS=()
# Forge carbide-api :1079 is plaintext gRPC in typical dev setups
if [[ "${SEC:-}" == "0" ]]; then
  PLAINTEXT_ARGS=(-plaintext)
elif [[ -z "${SEC// }" ]]; then
  warn "Probing with -plaintext because CARBIDE_SEC_OPT is unset (set to 0 if this matches your Core)."
  PLAINTEXT_ARGS=(-plaintext)
else
  die "CARBIDE_SEC_OPT=${SEC} — automated grpcurl for TLS/mTLS not in this script. Test manually with grpcurl -cacert/-cert/-key, or set CARBIDE_SEC_OPT=0 for plaintext."
fi

PODNAME="grpc-carbide-verify-$RANDOM"
info "grpcurl probe: ${PLAINTEXT_ARGS[*]} $TARGET forge.Forge/Version (pod $PODNAME in $RUN_NS)"

kubectl delete pod -n "$RUN_NS" "$PODNAME" --ignore-not-found --wait=false >/dev/null 2>&1 || true
sleep 1

kubectl run "$PODNAME" -n "$RUN_NS" --restart=Never --image=fullstorydev/grpcurl:latest \
  --command -- /bin/grpcurl -connect-timeout 30 "${PLAINTEXT_ARGS[@]}" "$TARGET" forge.Forge/Version

for _ in $(seq 1 60); do
  phase=$(kubectl get pod -n "$RUN_NS" "$PODNAME" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
  if [[ "$phase" == "Succeeded" || "$phase" == "Failed" ]]; then
    break
  fi
  sleep 1
done

echo "--- grpcurl pod status / logs ---"
kubectl get pod -n "$RUN_NS" "$PODNAME" -o wide 2>/dev/null || true
kubectl logs -n "$RUN_NS" "$PODNAME" 2>/dev/null || true
kubectl delete pod -n "$RUN_NS" "$PODNAME" --wait=false >/dev/null 2>&1 || true

echo ""
ok_msg="If grpcurl logs show a Version response, Core is reachable with plaintext gRPC (site-agent should use CARBIDE_SEC_OPT=0)."
echo "OK: $ok_msg"
echo "    To re-issue SPIFFE client TLS secret (mTLS to REST PKI path): ${REPO_ROOT}/scripts/setup-site-agent-core-grpc-certs.sh"
