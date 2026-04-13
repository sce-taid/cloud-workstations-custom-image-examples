#!/bin/bash

# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# This script centralizes workstation configuration.
# It can be called by any image layer to perform common setup tasks.

# Source the integration utility
# shellcheck source=/dev/null
source /google/scripts/build/desktop_integration.sh

configure_apt() {
  local region="${GCP_REGION:-us-central1}"
  echo "Configuring APT repositories for region: ${region}..."

  # Ensure correct permissions for APT configs and keys
  find /etc/apt/sources.list.d/ -type f -name "*.list" -exec chmod 644 {} + 2>/dev/null || true
  find /etc/apt/keyrings/ -type f -exec chmod 644 {} + 2>/dev/null || true

  # Replace default region in all .list files
  sed -i "s/us-central1/${region}/g" /etc/apt/sources.list.d/*.list 2>/dev/null || true
}

prepare_assets() {
  echo "Applying standard asset permissions..."
  # Fix permissions for common asset locations that might be merged from child layers
  [ -d "/build-hooks.d" ] && find /build-hooks.d -type f -exec chmod +x {} + 2>/dev/null || true
  [ -d "/google/scripts" ] && find /google/scripts -type f -exec chmod +x {} + 2>/dev/null || true
  [ -d "/etc/workstation-startup.d" ] && find /etc/workstation-startup.d -type f -exec chmod +x {} + 2>/dev/null || true
  [ -d "/etc/xdg/autostart" ] && find /etc/xdg/autostart -type f -exec chmod 644 {} + 2>/dev/null || true
  [ -d "/usr/share/applications" ] && find /usr/share/applications -type f -exec chmod 644 {} + 2>/dev/null || true
}

install_packages() {
  if [[ -n "${EXTRA_PKGS:-}" ]]; then
    echo "Installing extra packages: ${EXTRA_PKGS}..."
    # shellcheck disable=SC2086
    apt-get install -y --no-install-recommends ${EXTRA_PKGS}
  fi
}

install_debs() {
  if [[ -n "${EXTRA_DEB_URLS:-}" ]]; then
    echo "Installing extra .deb packages from URLs..."
    for url in ${EXTRA_DEB_URLS}; do
      local filename=$(basename "${url}")
      echo "  -> Downloading and installing ${filename}"
      curl ${CURL_OPTS:--fsSL --retry 3} -O "${url}"
      apt-get install -y ./"${filename}"
      rm "${filename}"
    done
  fi
}

run_hooks() {
  if [ -d "/build-hooks.d/" ]; then
    for hook in $(find /build-hooks.d/ -name '*.sh' -print | sort); do
      [ -e "$hook" ] || continue
      echo "Running build hook: $hook"
      /bin/bash "$hook"
    done
  fi
}

main() {
  prepare_assets
  configure_apt
  run_hooks

  echo "Updating package lists..."
  apt-get update

  # Ensure artifact registry transport is present if any 'ar+' repos were added
  if grep -q "ar+" /etc/apt/sources.list.d/*.list 2>/dev/null; then
    echo "Installing apt-transport-artifact-registry for specialized repositories..."
    # We must install the transport first to allow subsequent updates to work
    apt-get install -y --no-install-recommends apt-transport-artifact-registry
    apt-get update
  fi

  install_packages
  install_debs
  desktop_apply_integration

  echo "Cleaning up..."
  apt-get autoremove -y
  apt-get clean
  rm -rf /var/lib/apt/lists/*
}

main "$@"
