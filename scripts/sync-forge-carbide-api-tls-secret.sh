#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Copy the Forge carbide-api TLS secret (same material as grpcurl -cacert/-cert/-key)
# into the site-agent namespace so the StatefulSet can mount it at /etc/carbide as
# ca.crt, tls.crt, tls.key.
#
# Typical grpcurl pod used:
#   secretName: carbide-api-certificate (namespace forge-system)
#   mountPath: /certs -> ca.crt, tls.crt, tls.key
#
# After this script, set Helm values (or patch StatefulSet):
#   secrets.carbideTlsCerts: <DEST_SECRET>
# Optionally disable cert-manager site-agent client cert when you rely only on this bundle:
#   certificate.enabled: false
#
# Usage:
#   ./scripts/sync-forge-carbide-api-tls-secret.sh
#   ./scripts/sync-forge-carbide-api-tls-secret.sh --dest-ns carbide-rest --dest-secret carbide-api-grpc-client
#   ./scripts/sync-forge-carbide-api-tls-secret.sh --source-ns forge-system --source-secret carbide-api-certificate
#
set -euo pipefail

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }
ok() { echo "OK: $*"; }

SOURCE_NS="${SOURCE_NS:-forge-system}"
SOURCE_SECRET="${SOURCE_SECRET:-carbide-api-certificate}"
DEST_NS="${DEST_NS:-carbide-rest}"
DEST_SECRET="${DEST_SECRET:-carbide-api-grpc-client}"
DRY_RUN=false

usage() {
  sed -n '1,35p' "$0" | tail -n +2
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-ns)      SOURCE_NS="${2:?}"; shift 2 ;;
    --source-secret)  SOURCE_SECRET="${2:?}"; shift 2 ;;
    --dest-ns)        DEST_NS="${2:?}"; shift 2 ;;
    --dest-secret)    DEST_SECRET="${2:?}"; shift 2 ;;
    --dry-run)        DRY_RUN=true; shift ;;
    -h|--help)        usage ;;
    *) die "Unknown option: $1 (use --help)" ;;
  esac
done

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }
need_cmd kubectl
kubectl cluster-info >/dev/null 2>&1 || die "kubectl cannot reach the cluster"

kubectl get secret "$SOURCE_SECRET" -n "$SOURCE_NS" >/dev/null 2>&1 || \
  die "Secret $SOURCE_SECRET not found in namespace $SOURCE_NS"

TMPDIR="$(mktemp -d)"
cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

extract() {
  local key="$1"
  local out="$2"
  kubectl get secret "$SOURCE_SECRET" -n "$SOURCE_NS" -o "jsonpath={.data.$key}" | base64 -d >"$out" || die "missing key $key in secret $SOURCE_SECRET"
}

info "Extracting tls.crt, tls.key, ca.crt from $SOURCE_NS/$SOURCE_SECRET"
extract 'tls\.crt' "$TMPDIR/tls.crt"
extract 'tls\.key' "$TMPDIR/tls.key"
# ca.crt is optional on some secrets; try common key names
if kubectl get secret "$SOURCE_SECRET" -n "$SOURCE_NS" -o "jsonpath={.data.ca\.crt}" | grep -q .; then
  extract 'ca\.crt' "$TMPDIR/ca.crt"
elif kubectl get secret "$SOURCE_SECRET" -n "$SOURCE_NS" -o "jsonpath={.data.ca-bundle\.crt}" | grep -q .; then
  kubectl get secret "$SOURCE_SECRET" -n "$SOURCE_NS" -o "jsonpath={.data.ca-bundle\.crt}" | base64 -d >"$TMPDIR/ca.crt"
else
  die "Secret has no data.ca.crt (or ca-bundle.crt). Add CA material or use a tls type secret with ca.crt."
fi

if ! kubectl get namespace "$DEST_NS" >/dev/null 2>&1; then
  info "Creating namespace $DEST_NS"
  kubectl create namespace "$DEST_NS"
fi

K_CREATE=(kubectl create secret generic "$DEST_SECRET" -n "$DEST_NS" \
  --from-file=tls.crt="$TMPDIR/tls.crt" \
  --from-file=tls.key="$TMPDIR/tls.key" \
  --from-file=ca.crt="$TMPDIR/ca.crt")

if [[ "$DRY_RUN" == true ]]; then
  info "Dry-run: ${K_CREATE[*]} --dry-run=client -o yaml"
  "${K_CREATE[@]}" --dry-run=client -o yaml
  exit 0
fi

info "Applying secret $DEST_NS/$DEST_SECRET (keys: tls.crt, tls.key, ca.crt)"
kubectl delete secret "$DEST_SECRET" -n "$DEST_NS" --ignore-not-found >/dev/null 2>&1 || true
"${K_CREATE[@]}"

ok "Secret $DEST_NS/$DEST_SECRET ready (same layout as site-agent /etc/carbide mount)"

cat <<EOF

Next steps:
  1) Helm values.yaml (or -f override):
       secrets:
         carbideTlsCerts: $DEST_SECRET
       # Optional: stop issuing a duplicate client cert from this chart:
       # certificate:
       #   enabled: false

  2) Upgrade / apply and restart site-agent:
       helm upgrade ...  # your release
       kubectl rollout restart statefulset/carbide-rest-site-agent -n $DEST_NS

  Site-agent defaults already use CARBIDE_SEC_OPT=2 and paths /etc/carbide/ca.crt, tls.crt, tls.key.
EOF
