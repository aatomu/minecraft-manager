#!/bin/bash
# usage: ./call_protected.sh <password> [server_url] [api_path] [body]
PASSWORD="${1:-}"
SERVER_URL="${2:-http://localhost:80}"
API_PATH="${3:-/api/data}"
BODY="${4:-}"

if [ -z "$PASSWORD" ]; then
  echo "Usage: $0 <password> [server_url] [api_path] [body]"
  exit 2
fi

AUTH_RAW=$(curl -sS "${SERVER_URL}/new_token")
if [ -z "$AUTH_RAW" ]; then
  echo "failed to get new_token from ${SERVER_URL}/new_token" >&2
  exit 3
fi

IFS=',' read -r ID KEY <<<"$AUTH_RAW"
if [ -z "$ID" ] || [ -z "$KEY" ]; then
  echo "invalid token response: ${AUTH_RAW}" >&2
  exit 4
fi

HASH=$(echo -n "${ID}${PASSWORD}" | openssl dgst -sha512 -mac HMAC -macopt hexkey:"${KEY}" | awk '{print $2}')
if [ -z "$HASH" ]; then
  echo "failed to compute HMAC" >&2
  exit 5
fi

AUTH_HEADER="${ID}:${HASH}"

if [ -z "$BODY" ]; then
  curl -i -H "Authorization: ${AUTH_HEADER}" "${SERVER_URL}${API_PATH}"
else
  curl -i -X POST -H "Authorization: ${AUTH_HEADER}" -H "Content-Type: application/json" -d "${BODY}" "${SERVER_URL}${API_PATH}"
fi
