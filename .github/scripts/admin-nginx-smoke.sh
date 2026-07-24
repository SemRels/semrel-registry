#!/usr/bin/env bash

set -Eeuo pipefail

case "$(uname -s)" in
  MINGW*|MSYS*) export MSYS_NO_PATHCONV=1 ;;
esac

script_dir="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(CDPATH= cd -- "$script_dir/../.." && pwd)"
cd "$repo_root"

image="${ADMIN_SMOKE_IMAGE:-semrel-registry-admin:smoke}"
build_image=1

usage() {
  cat <<'EOF'
Usage: admin-nginx-smoke.sh [--image IMAGE] [--skip-build]

Build and smoke test the admin image with delayed API DNS registration.
Set CONTAINER_ENGINE=docker or CONTAINER_ENGINE=podman to select an engine.
Set PODMAN_CONNECTION when a non-default Podman connection is required.
EOF
}

while (($#)); do
  case "$1" in
    --image)
      [[ $# -ge 2 ]] || { echo "error: --image requires a value" >&2; exit 2; }
      image="$2"
      shift 2
      ;;
    --skip-build)
      build_image=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -n "${CONTAINER_ENGINE:-}" ]]; then
  engine="$CONTAINER_ENGINE"
elif command -v docker >/dev/null 2>&1; then
  engine=docker
elif command -v podman >/dev/null 2>&1; then
  engine=podman
else
  echo "error: docker or podman is required" >&2
  exit 1
fi

command -v "$engine" >/dev/null 2>&1 || {
  echo "error: container engine '$engine' was not found" >&2
  exit 1
}

engine_args=()
if [[ "$engine" == "podman" && -n "${PODMAN_CONNECTION:-}" ]]; then
  engine_args=(--connection "$PODMAN_CONNECTION")
fi

container() {
  "$engine" "${engine_args[@]}" "$@"
}

suffix="$(date +%s)-$$-${RANDOM:-0}"
network="semrel-admin-smoke-net-$suffix"
admin="semrel-admin-smoke-admin-$suffix"
backend="semrel-admin-smoke-api-$suffix"
network_created=0
admin_created=0
backend_created=0

cleanup() {
  set +e
  ((backend_created)) && container rm --force "$backend" >/dev/null 2>&1
  ((admin_created)) && container rm --force "$admin" >/dev/null 2>&1
  ((network_created)) && container network rm "$network" >/dev/null 2>&1
}
trap cleanup EXIT
trap 'exit 130' INT TERM

fail() {
  echo "error: $*" >&2
  if ((admin_created)); then
    echo "--- admin logs ---" >&2
    container logs "$admin" >&2 || true
  fi
  if ((backend_created)); then
    echo "--- backend logs ---" >&2
    container logs "$backend" >&2 || true
  fi
  exit 1
}

is_running() {
  [[ "$(container inspect --format '{{.State.Running}}' "$1" 2>/dev/null)" == "true" ]]
}

if ((build_image)); then
  echo "Building $image"
  build_args=()
  [[ "$engine" == "podman" ]] && build_args=(--format docker)
  container build "${build_args[@]}" --quiet --file admin/Dockerfile --tag "$image" .
fi

network_created=1
container network create "$network" >/dev/null

echo "Starting admin before the api hostname exists"
admin_created=1
container run --detach --name "$admin" --network "$network" "$image" >/dev/null

sleep 2
is_running "$admin" || fail "admin exited before the API was available"
container exec "$admin" nginx -t >/dev/null 2>&1 ||
  fail "generated nginx configuration is invalid"

filter="$(container exec "$admin" printenv NGINX_ENVSUBST_FILTER)"
[[ "$filter" == '^(API_URL|NGINX_LOCAL_RESOLVERS)$' ]] ||
  fail "unexpected NGINX_ENVSUBST_FILTER: $filter"

generated_config="$(container exec "$admin" cat /etc/nginx/conf.d/default.conf)"
resolvers="$(container exec "$admin" awk \
  'BEGIN{ORS=" "} $1=="nameserver" {if ($2 ~ ":") {print "["$2"]"} else {print $2}}' \
  /etc/resolv.conf)"
resolvers="${resolvers% }"
[[ -n "$resolvers" ]] || fail "official nginx entrypoint did not discover a resolver"

if grep -Eq '\$\{(API_URL|NGINX_LOCAL_RESOLVERS)\}' <<<"$generated_config"; then
  fail "startup placeholders remain in generated nginx configuration"
fi
grep -Fq "resolver $resolvers valid=5s;" <<<"$generated_config" ||
  fail "generated nginx configuration does not use the discovered resolver"
grep -Fq 'set $api_url http://api:8080;' <<<"$generated_config" ||
  fail "API_URL was not substituted into the runtime upstream variable"
[[ "$(grep -Fc 'proxy_pass $api_url;' <<<"$generated_config")" -eq 2 ]] ||
  fail "both API proxy locations must use the runtime upstream variable"

for directive in \
  'proxy_set_header Host $host;' \
  'proxy_set_header X-Real-IP $remote_addr;' \
  'proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;' \
  'proxy_set_header X-Forwarded-Proto $scheme;'
do
  [[ "$(grep -Fc "$directive" <<<"$generated_config")" -eq 2 ]] ||
    fail "nginx runtime variable was altered: $directive"
done

set +e
before_output="$(container exec "$admin" wget -S -T 5 -O /dev/null \
  'http://127.0.0.1/api/before-backend?probe=missing' 2>&1)"
before_status=$?
set -e
((before_status != 0)) || fail "request unexpectedly succeeded without an API hostname"
grep -Eq 'HTTP/[0-9.]+ 502 ' <<<"$before_output" ||
  fail "request before API registration did not return 502: $before_output"
is_running "$admin" || fail "admin exited after the expected 502 response"

backend_command=$(cat <<'BACKEND_COMMAND'
cat > /etc/nginx/conf.d/default.conf <<'NGINX_CONFIG'
server {
    listen 8080;
    location / {
        default_type text/plain;
        return 200 "uri=$request_uri|host=$host|real=$http_x_real_ip|forwarded=$http_x_forwarded_for|proto=$http_x_forwarded_proto";
    }
}
NGINX_CONFIG
exec nginx -g 'daemon off;'
BACKEND_COMMAND
)

echo "Registering a minimal backend with the api network alias"
backend_created=1
container run --detach --name "$backend" --network "$network" \
  --network-alias api --entrypoint /bin/sh "$image" -c "$backend_command" >/dev/null

sleep 6
is_running "$backend" || fail "minimal API backend failed to start"

request_through_admin() {
  local path="$1"
  local response
  local attempt

  for attempt in {1..15}; do
    if response="$(container exec "$admin" wget -q -T 5 -O - \
      "http://127.0.0.1$path" 2>/dev/null)"; then
      printf '%s' "$response"
      return 0
    fi
    sleep 1
  done

  fail "admin did not recover after API DNS registration for $path"
}

forwarded='host=127.0.0.1|real=127.0.0.1|forwarded=127.0.0.1|proto=http'
api_response="$(request_through_admin '/api/runtime-dns?probe=api')"
[[ "$api_response" == "uri=/api/runtime-dns?probe=api|$forwarded" ]] ||
  fail "/api/ URI or forwarding headers changed: $api_response"

schemas_response="$(request_through_admin '/schemas/runtime-dns.json?probe=schemas')"
[[ "$schemas_response" == "uri=/schemas/runtime-dns.json?probe=schemas|$forwarded" ]] ||
  fail "/schemas/ URI or forwarding headers changed: $schemas_response"

is_running "$admin" || fail "admin stopped after backend recovery"
echo "Admin runtime DNS smoke test passed"
