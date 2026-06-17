#!/usr/bin/env bash
set -euo pipefail

backend_timeout_seconds="${BACKEND_TIMEOUT_SECONDS:-120}"
frontend_timeout_seconds="${FRONTEND_TIMEOUT_SECONDS:-120}"
build_flag="--build"

if [[ "${1:-}" == "--no-build" ]]; then
  build_flag=""
fi

step() {
  printf '==> %s\n' "$1"
}

note() {
  printf '%s\n' "$1"
}

trim() {
  local value="$1"

  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"

  printf '%s' "${value}"
}

random_token() {
  od -An -N24 -tx1 /dev/urandom | tr -d ' \n'
}

get_env_value() {
  local key="$1"
  local match

  match="$(grep -E "^${key}=" "$env_file" | tail -n 1 || true)"
  if [[ -z "${match}" ]]; then
    printf ''
    return
  fi

  printf '%s' "${match#*=}"
}

set_env_value() {
  local key="$1"
  local value="$2"

  if grep -qE "^${key}=" "$env_file"; then
    sed -i.bak -E "s|^${key}=.*$|${key}=${value}|" "$env_file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$env_file"
  fi
}

needs_randomization() {
  local current="$1"
  shift || true

  if [[ -z "${current}" ]]; then
    return 0
  fi

  for candidate in "$@"; do
    if [[ "${current}" == "${candidate}" ]]; then
      return 0
    fi
  done

  return 1
}

append_csv_unique() {
  local csv="$1"
  local item
  local entry
  local result=''
  local found=0
  local -a entries=()

  item="$(trim "${2:-}")"
  if [[ -z "${item}" ]]; then
    printf '%s' "$(trim "${csv}")"
    return
  fi

  if [[ -n "${csv}" ]]; then
    IFS=',' read -r -a entries <<<"${csv}"
  fi

  for entry in "${entries[@]}"; do
    entry="$(trim "${entry}")"
    if [[ -z "${entry}" ]]; then
      continue
    fi
    if [[ "${entry}" == "${item}" ]]; then
      found=1
    fi
    if [[ -z "${result}" ]]; then
      result="${entry}"
    else
      result="${result},${entry}"
    fi
  done

  if (( found == 0 )); then
    if [[ -z "${result}" ]]; then
      result="${item}"
    else
      result="${result},${item}"
    fi
  fi

  printf '%s' "${result}"
}

append_csv_list_unique() {
  local csv="$1"
  local list="$2"
  local entry
  local result="${csv}"
  local -a entries=()

  if [[ -z "$(trim "${list}")" ]]; then
    printf '%s' "$(trim "${result}")"
    return
  fi

  IFS=',' read -r -a entries <<<"${list}"
  for entry in "${entries[@]}"; do
    result="$(append_csv_unique "${result}" "${entry}")"
  done

  printf '%s' "${result}"
}

detect_host_accessible_ip() {
  if ! command -v ip >/dev/null 2>&1; then
    return
  fi

  ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i=1;i<=NF;i++) if ($i=="src") { print $(i+1); exit }}'
}

is_wsl() {
  if [[ -n "${WSL_DISTRO_NAME:-}" ]]; then
    return 0
  fi

  grep -qi microsoft /proc/sys/kernel/osrelease 2>/dev/null
}

start_wsl_keepalive() {
  local keepalive_enabled="${KEEP_WSL_ALIVE:-1}"
  local keepalive_script="${script_dir}/wsl-keepalive.sh"
  local keepalive_log='/tmp/agentroom-wsl-keepalive.log'

  if [[ "${keepalive_enabled}" == '0' ]] || ! is_wsl; then
    return
  fi

  if pgrep -f "${keepalive_script}" >/dev/null 2>&1; then
    return
  fi

  nohup bash "${keepalive_script}" >"${keepalive_log}" 2>&1 </dev/null &
  note 'WSL keepalive is running so the Docker host does not idle out. Set KEEP_WSL_ALIVE=0 before startup if you want to disable it.'
}

fetch_url() {
  local url="$1"

  if command -v curl >/dev/null 2>&1; then
    curl -fsS "$url"
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
    return
  fi

  printf 'curl or wget is required to check service readiness.\n' >&2
  exit 1
}

extract_published_port() {
  local mapping="$1"
  local service="$2"
  local container_port="$3"

  if [[ -z "${mapping}" ]]; then
    printf 'Could not determine the published port for %s:%s.\n' "${service}" "${container_port}" >&2
    exit 1
  fi

  printf '%s' "${mapping##*:}"
}

resolve_backend_port() {
  local mapping

  mapping="$(docker compose port backend 8080 2>/dev/null | head -n 1 | tr -d '\r')"
  extract_published_port "${mapping}" 'backend' '8080'
}

resolve_frontend_port() {
  local mapping

  mapping="$(docker compose port frontend 80 2>/dev/null | head -n 1 | tr -d '\r')"
  extract_published_port "${mapping}" 'frontend' '80'
}

wait_for_backend() {
  local backend_health_url="$1"
  local deadline=$((SECONDS + backend_timeout_seconds))

  while (( SECONDS < deadline )); do
    if fetch_url "${backend_health_url}" | grep -q '"ok":true'; then
      return
    fi

    sleep 2
  done

  printf 'Backend health check never passed at %s within %s seconds.\n' "${backend_health_url}" "$backend_timeout_seconds" >&2
  exit 1
}

wait_for_frontend() {
  local frontend_url="$1"
  local deadline=$((SECONDS + frontend_timeout_seconds))

  while (( SECONDS < deadline )); do
    if fetch_url "${frontend_url}" >/dev/null 2>&1; then
      return
    fi

    sleep 2
  done

  printf 'Frontend never answered at %s within %s seconds.\n' "${frontend_url}" "$frontend_timeout_seconds" >&2
  exit 1
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
env_file="${repo_root}/.env"
env_example_file="${repo_root}/.env.example"

cd "${repo_root}"

if ! command -v docker >/dev/null 2>&1; then
  printf 'Docker CLI is not installed or is not on PATH.\n' >&2
  exit 1
fi

step 'Checking Docker daemon'
if ! docker info >/dev/null 2>&1; then
  printf 'Docker daemon is unavailable. Start Docker Desktop or dockerd and rerun this script.\n' >&2
  exit 1
fi

if [[ ! -f "${env_file}" ]]; then
  step 'Bootstrapping .env from .env.example'
  cp "${env_example_file}" "${env_file}"
fi

admin_api_key="$(get_env_value 'ADMIN_API_KEY')"
if needs_randomization "${admin_api_key}" 'change_me_admin_key'; then
  admin_api_key="$(random_token)"
fi
set_env_value 'ADMIN_API_KEY' "${admin_api_key}"
set_env_value 'VITE_ADMIN_API_KEY' "${admin_api_key}"

mysql_password="$(get_env_value 'MYSQL_PASSWORD')"
if needs_randomization "${mysql_password}" 'agentroom_password'; then
  mysql_password="$(random_token)"
fi
set_env_value 'MYSQL_PASSWORD' "${mysql_password}"

mysql_root_password="$(get_env_value 'MYSQL_ROOT_PASSWORD')"
if needs_randomization "${mysql_root_password}" 'change_me_root_password'; then
  mysql_root_password="$(random_token)"
fi
set_env_value 'MYSQL_ROOT_PASSWORD' "${mysql_root_password}"

llm_api_key="$(get_env_value 'LLM_API_KEY')"
if [[ "${llm_api_key}" == 'your-api-key-here' ]]; then
  set_env_value 'LLM_API_KEY' ''
fi

public_origins="$(get_env_value 'PUBLIC_ORIGIN')"
if [[ -n "${PUBLIC_ORIGIN:-}" ]]; then
  public_origins="${PUBLIC_ORIGIN}"
fi
public_origins="$(trim "${public_origins}")"
if [[ -n "${public_origins}" ]]; then
  set_env_value 'PUBLIC_ORIGIN' "${public_origins}"
fi

direct_access_ip="$(detect_host_accessible_ip)"
direct_frontend_url=''
direct_backend_health_url=''

allowed_origins="$(get_env_value 'ALLOWED_ORIGINS')"
allowed_origins="$(append_csv_unique "${allowed_origins}" 'http://localhost:5173')"
allowed_origins="$(append_csv_unique "${allowed_origins}" 'http://127.0.0.1:5173')"
allowed_origins="$(append_csv_list_unique "${allowed_origins}" "${public_origins}")"
if [[ -n "${direct_access_ip}" && "${direct_access_ip}" != '127.0.0.1' ]]; then
  direct_frontend_url="http://${direct_access_ip}:5173"
  direct_backend_health_url="http://${direct_access_ip}:8080/api/health"
  allowed_origins="$(append_csv_unique "${allowed_origins}" "${direct_frontend_url}")"
fi
set_env_value 'ALLOWED_ORIGINS' "${allowed_origins}"

rm -f "${env_file}.bak"

step 'Validating docker compose configuration'
docker compose config >/dev/null

step 'Starting AgentRoom stack'
if [[ -n "${build_flag}" ]]; then
  docker compose up -d --build
else
  docker compose up -d
fi

backend_port="$(resolve_backend_port)"
frontend_port="$(resolve_frontend_port)"
frontend_url="http://127.0.0.1:${frontend_port}"
backend_health_url="http://127.0.0.1:${backend_port}/api/health"

direct_frontend_url=''
direct_backend_health_url=''
if [[ -n "${direct_access_ip}" && "${direct_access_ip}" != '127.0.0.1' ]]; then
  direct_frontend_url="http://${direct_access_ip}:${frontend_port}"
  direct_backend_health_url="http://${direct_access_ip}:${backend_port}/api/health"
fi

current_allowed_origins="$(trim "$(get_env_value 'ALLOWED_ORIGINS')")"
updated_allowed_origins="$(append_csv_unique "${current_allowed_origins}" "http://localhost:${frontend_port}")"
updated_allowed_origins="$(append_csv_unique "${updated_allowed_origins}" "http://127.0.0.1:${frontend_port}")"
if [[ -n "${direct_frontend_url}" ]]; then
  updated_allowed_origins="$(append_csv_unique "${updated_allowed_origins}" "${direct_frontend_url}")"
fi
if [[ "${updated_allowed_origins}" != "${current_allowed_origins}" ]]; then
  step 'Refreshing backend origin allowlist'
  set_env_value 'ALLOWED_ORIGINS' "${updated_allowed_origins}"
  rm -f "${env_file}.bak"
  docker compose up -d --force-recreate --no-deps backend
fi

step 'Waiting for backend health'
wait_for_backend "${backend_health_url}"

step 'Waiting for frontend'
wait_for_frontend "${frontend_url}"

step 'Container status'
docker compose ps

final_llm_api_key="$(get_env_value 'LLM_API_KEY')"
if [[ -z "${final_llm_api_key}" ]]; then
  note 'LLM_API_KEY is blank. Human chat will work, but agent replies stay disabled until you set a real key and rerun this script.'
fi
start_wsl_keepalive

printf '\nAgentRoom is ready.\n'
printf 'Frontend: http://localhost:%s\n' "${frontend_port}"
if [[ -n "${direct_frontend_url}" ]]; then
  printf 'Frontend (direct IP): %s\n' "${direct_frontend_url}"
fi
printf 'Backend health: http://localhost:%s/api/health\n' "${backend_port}"
if [[ -n "${direct_backend_health_url}" ]]; then
  printf 'Backend health (direct IP): %s\n' "${direct_backend_health_url}"
fi
