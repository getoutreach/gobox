#!/usr/bin/env bash
# This script should be run to provide a local configuration file if you intend to
# build/run/debug your service directly, outside of the kubernetes dev-environment.

set -e
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
overridePath="$(dirname "$0")/devconfig.override.sh"
configDir="$HOME/.outreach/gobox"

# shellcheck source=./lib/logging.sh
source "$DIR/lib/logging.sh"

mkdir -p "$configDir"

export VAULT_ADDR=https://vault.outreach.cloud

ensure_logged_into_vault() {

  # We redirect to stderr here to prevent mangling the output when used
  {
    # Attempt to log into vault if we aren't already.
    if ! vault kv list dev >/dev/null 2>&1; then
      local vaultVersion
      local vaultMajorVersion
      local vaultMinorVersion
      vaultVersion="$(vault version | awk '{ print $2 }' | sed -e 's:^v::g')"
      vaultMajorVersion="$(cut -d. -f1 <<<"$vaultVersion")"
      vaultMinorVersion="$(cut -d. -f2 <<<"$vaultVersion")"
      if [[ $vaultMajorVersion -lt 1 || ($vaultMajorVersion -eq 1 && $vaultMinorVersion -lt 1) ]]; then
        fatal "Please upgrade the Vault CLI. Try running 'outreach k8s install_deps'"
      fi
      info "Logging user into vault"
      vault login -method=oidc
    fi
  } 1>&2
}

get_vault_secrets() {
  # Should be path/to/key, this is fed into vault
  local key="$1"

  # Path to store the secrets at
  local path="$2"
  path="$path/$(basename "$key")"

  mkdir -p "$path"

  ensure_logged_into_vault

  # shellcheck disable=SC2155
  local data="$(vault kv get -format=json "$key" | jq -cr '.data.data')"
  if [[ -z $data ]]; then
    fatal "Failed to get vault key '$key'"
  fi

  mapfile -t subKeys < <(jq -r 'keys[]' <<<"$data")
  for subKey in "${subKeys[@]}"; do
    jq -cr ".[\"$subKey\"]" <<<"$data" | sed 's/\n//' | tr -d '\n' >"$path/$subKey"
  done

  return 0
}

ensure_logged_into_vault

info "Generating local config/secrets in '$configDir'"
info_sub "fetching secret 'dev/devenv/honeycomb'"
get_vault_secrets "dev/devenv/honeycomb" "$configDir"

# We add logfmt.yaml directly here because this is only needed for local development.
# This is not meant to be used in any kubernetes setup
info "Configuring logfmt"
mkdir -p "$HOME/.outreach/logfmt"
get_vault_secrets "dev/datadog/dev-env" "$HOME/.outreach/logfmt"
cat >"$HOME/.outreach/logfmt/logfmt.yaml" <<EOF
DatadogAPIKey:
  Path: "$HOME/.outreach/logfmt/dev-env/api_key"
EOF

# Look for a override script that allows users to extend this process outside of bootstrap
if [[ -e $overridePath ]]; then
  # shellcheck disable=SC1090
  source "$overridePath"
fi
