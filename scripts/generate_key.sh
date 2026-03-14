#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <organization_id> <cluster_id> [key_name]"
  echo "Required env vars: KUBEROOT_BACKEND_URL, INTERNAL_API_TOKEN"
  exit 1
fi

if [[ -z "${KUBEROOT_BACKEND_URL:-}" ]]; then
  echo "KUBEROOT_BACKEND_URL is required"
  exit 1
fi

if [[ -z "${INTERNAL_API_TOKEN:-}" ]]; then
  echo "INTERNAL_API_TOKEN is required"
  exit 1
fi

organization_id="$1"
cluster_id="$2"
key_name="${3:-${organization_id}-${cluster_id}-agent}"

tmp_response_file=$(mktemp)
http_status=$(curl -sS -o "$tmp_response_file" -w "%{http_code}" -X POST "${KUBEROOT_BACKEND_URL%/}/internal/generate-key" \
  -H "Content-Type: application/json" \
  -H "X-Internal-Token: ${INTERNAL_API_TOKEN}" \
  -d "{\"organizationId\":\"${organization_id}\",\"name\":\"${key_name}\",\"clusterId\":\"${cluster_id}\"}")

response=$(cat "$tmp_response_file")
rm -f "$tmp_response_file"

if [[ "$http_status" != "201" ]]; then
  echo "Failed to generate key (HTTP ${http_status})"
  echo "$response"
  if [[ "$http_status" == "401" ]]; then
    echo "Hint: INTERNAL_API_TOKEN must be backend's internal admin token, not a generated kr_live API key."
  fi
  exit 1
fi

api_key=$(printf '%s' "$response" | python3 -c 'import json,sys
try:
    data=json.load(sys.stdin)
except Exception:
    print("")
    raise SystemExit(0)
print(data.get("apiKey",""))')

if [[ -z "$api_key" ]]; then
  echo "Failed to parse apiKey from response"
  echo "$response"
  exit 1
fi

cat <<EOF
API key generated successfully.

Organization: ${organization_id}
Cluster ID:   ${cluster_id}
Key name:     ${key_name}
API Key:      ${api_key}

Install command to send:

export KUBEROOT_API_KEY=${api_key}
export KUBEROOT_CLUSTER_ID=${cluster_id}

curl -sSL https://raw.githubusercontent.com/LikhithSM/KubeRoot/refs/heads/main/install.yaml \\
  | envsubst \\
  | kubectl apply -f -
EOF
