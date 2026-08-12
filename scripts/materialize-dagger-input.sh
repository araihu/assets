#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: materialize-dagger-input.sh MODE OUTPUT\n' >&2
  exit 2
}

fail() {
  printf 'materialize-dagger-input: %s\n' "$1" >&2
  exit 1
}

[[ $# -eq 2 ]] || usage
mode=$1
output=$2
strict_semver='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

[[ -n "$output" ]] || fail 'output path is required'
[[ "$output" != *$'\n'* && "$output" != *$'\r'* ]] || fail 'output path contains control characters'

matches() {
  local name=$1 value=$2 pattern=$3
  [[ "$value" =~ $pattern ]] || fail "$name has an invalid value"
}

read_provider_env() {
  local destination=$1 name=$2 value
  [[ -v "$name" ]] || fail "$name is required"
  value=${!name}
  [[ "$value" != *$'\n' ]] || fail "$name must not end with LF"
  printf -v "$destination" '%s' "$value"
}

json_quote() {
  local value=$1 out='"' char i
  for ((i = 0; i < ${#value}; i++)); do
    char=${value:i:1}
    case "$char" in
      '"') out+="\\\"" ;;
      \\) out+="\\\\" ;;
      $'\b') out+="\\b" ;;
      $'\f') out+="\\f" ;;
      $'\n') out+="\\n" ;;
      $'\r') out+="\\r" ;;
      $'\t') out+="\\t" ;;
      *)
        [[ "$char" != [[:cntrl:]] ]] || fail 'provider value contains an unsupported control character'
        out+="$char"
        ;;
    esac
  done
  out+='"'
  printf '%s' "$out"
}

json_object() {
  local first=1 key value
  printf '{'
  while (( $# )); do
    key=$1
    value=$2
    shift 2
    (( first )) || printf ','
    first=0
    printf '%s:' "$(json_quote "$key")"
    json_quote "$value"
  done
  printf '}'
}

write_payload() {
  local directory tmp
  directory=${output%/*}
  [[ "$directory" == "$output" ]] && directory=.
  mkdir -p -m 700 "$directory"
  tmp=$(mktemp "${output}.tmp.XXXXXX")
  trap 'rm -f "$tmp"' EXIT
  chmod 0600 "$tmp"
  printf '%s\n' "$1" > "$tmp"
  mv -f "$tmp" "$output"
  trap - EXIT
}

read_provider_env event_name GITHUB_EVENT_NAME
read_provider_env run_id GITHUB_RUN_ID
read_provider_env run_attempt GITHUB_RUN_ATTEMPT
matches GITHUB_RUN_ID "$run_id" '^[1-9][0-9]*$'
matches GITHUB_RUN_ATTEMPT "$run_attempt" '^[1-9][0-9]*$'
run_nonce="$run_id-$run_attempt"

if [[ -n "${GITHUB_EVENT_PATH:-}" ]]; then
  [[ -s "$GITHUB_EVENT_PATH" && -r "$GITHUB_EVENT_PATH" ]] || fail 'GITHUB_EVENT_PATH is not a readable event file'
fi

case "$mode" in
  ci)
    case "$event_name" in
      pull_request) cache_namespace=pr ;;
      push|workflow_dispatch) cache_namespace=trusted ;;
      *) fail 'unsupported CI event' ;;
    esac
    payload=$(json_object cache_namespace "$cache_namespace" run_nonce "$run_nonce")
    ;;
  acquisition)
    [[ "$event_name" == workflow_dispatch ]] || fail 'acquisition requires workflow_dispatch'
    payload=$(json_object refresh_nonce "$run_nonce")
    ;;
  campaign-plan)
    [[ "$event_name" == schedule || "$event_name" == workflow_dispatch || "$event_name" == push ]] || fail 'unsupported campaign event'
    read_provider_env repository GITHUB_REPOSITORY
    read_provider_env owner AHAIRU_OWNER
    read_provider_env target_repository AHAIRU_REPOSITORY
    requested_date=${REQUESTED_DATE-}
    [[ "$requested_date" != *$'\n' ]] || fail 'REQUESTED_DATE must not end with LF'
    read_provider_env server_url GITHUB_SERVER_URL
    read_provider_env api_url GITHUB_API_URL
    payload=$(json_object \
      repository "$repository" \
      ahairu_owner "$owner" \
      ahairu_repository "$target_repository" \
      date "$requested_date" \
      github_server_url "$server_url" \
      github_api_url "$api_url" \
      refresh_nonce "$run_nonce")
    ;;
  campaign-dispatch)
    read_provider_env repository GITHUB_REPOSITORY
    read_provider_env owner AHAIRU_OWNER
    read_provider_env target_repository AHAIRU_REPOSITORY
    read_provider_env server_url GITHUB_SERVER_URL
    read_provider_env api_url GITHUB_API_URL
    read_provider_env artifact_id CHANNEL_ARTIFACT_ID
    read_provider_env artifact_url CHANNEL_ARTIFACT_URL
    read_provider_env artifact_sha256 CHANNEL_ARTIFACT_SHA256
    read_provider_env source_revision GITHUB_SHA
    matches CHANNEL_ARTIFACT_ID "$artifact_id" '^[1-9][0-9]*$'
    matches CHANNEL_ARTIFACT_SHA256 "$artifact_sha256" '^[0-9a-f]{64}$'
    matches GITHUB_SHA "$source_revision" '^[0-9a-f]{40}$'
    payload=$(json_object \
      repository "$repository" \
      ahairu_owner "$owner" \
      ahairu_repository "$target_repository" \
      github_server_url "$server_url" \
      github_api_url "$api_url" \
      artifact_id "$artifact_id" \
      artifact_url "$artifact_url" \
      artifact_sha256 "$artifact_sha256" \
      source_revision "$source_revision" \
      effect_nonce "$run_nonce")
    ;;
  release-build)
    [[ "$event_name" == push ]] || fail 'release build requires a tag push'
    read_provider_env ref_type GITHUB_REF_TYPE
    [[ "$ref_type" == tag ]] || fail 'release build requires a tag push'
    read_provider_env tag GITHUB_REF_NAME
    read_provider_env source_revision GITHUB_SHA
    read_provider_env repository GITHUB_REPOSITORY
    matches GITHUB_REF_NAME "$tag" "$strict_semver"
    matches GITHUB_SHA "$source_revision" '^[0-9a-f]{40}$'
    matches GITHUB_REPOSITORY "$repository" '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$'
    payload=$(json_object tag "$tag" source_revision "$source_revision" repository "$repository" refresh_nonce "$run_nonce")
    ;;
  release-publish)
    read_provider_env tag GITHUB_REF_NAME
    read_provider_env repository GITHUB_REPOSITORY
    matches GITHUB_REF_NAME "$tag" "$strict_semver"
    matches GITHUB_REPOSITORY "$repository" '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$'
    payload=$(json_object tag "$tag" repository "$repository" effect_nonce "$run_nonce")
    ;;
  fanout-plan)
    read_provider_env release RELEASE
    read_provider_env repository GITHUB_REPOSITORY
    matches RELEASE "$release" "$strict_semver"
    matches GITHUB_REPOSITORY "$repository" '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$'
    payload=$(json_object release "$release" repository "$repository" refresh_nonce "$run_nonce")
    ;;
  fanout-dispatch)
    payload=$(json_object effect_nonce "$run_nonce")
    ;;
  *)
    fail 'unsupported input mode'
    ;;
esac

write_payload "$payload"
