#!/bin/bash

# Copyright 2024-2026 Google LLC
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

# Sourced logic (from preflight or local)
# shellcheck disable=SC1091
source "/google/scripts/common.sh"

install_abfs() {
  local install_abfs_client="${INSTALL_ABFS_CLIENT:-false}"
  local curl_opts="${CURL_OPTS:--fsSL --retry 3}"
  local gpg_file="/usr/share/keyrings/cloud.google.gpg"

  if [[ "${install_abfs_client}" == "true" ]]; then
    echo "Configuring ABFS repositories..."
    # shellcheck disable=SC2086
    curl ${curl_opts} https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o "${gpg_file}"
    echo "deb [signed-by=${gpg_file}] https://packages.cloud.google.com/apt apt-transport-artifact-registry-stable main" \
      > /etc/apt/sources.list.d/artifact-registry.list
    echo "deb [signed-by=${gpg_file}] ar+https://us-apt.pkg.dev/projects/abfs-binaries abfs-apt-alpha-public main" \
      > /etc/apt/sources.list.d/abfs.list
    else
    echo "INSTALL_ABFS_CLIENT is false. Skipping ABFS configuration."
    fi
    }
main() {
  install_abfs
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
