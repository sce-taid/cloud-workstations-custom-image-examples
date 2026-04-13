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

# Key URLs
GOOGLE_CLOUD_SDK_KEY_URL="https://packages.cloud.google.com/apt/doc/apt-key.gpg"
DOCKER_KEY_URL="https://download.docker.com/linux/ubuntu/gpg"
YARN_KEY_URL="https://dl.yarnpkg.com/debian/pubkey.gpg"
GOOGLE_CHROME_KEY_URL="https://dl.google.com/linux/linux_signing_key.pub"
NODE_SOURCE_KEY_URL="https://deb.nodesource.com/gpgkey/nodesource.gpg.key"
ADOPTIUM_KEY_URL="https://packages.adoptium.net/artifactory/api/gpg/key/public"
NVIDIA_KEY_URL="https://nvidia.github.io/libnvidia-container/gpgkey"

# Target directory
TARGET_DIR="examples/images/gnome/assets/etc/apt/keyrings"

# Create target directory if it doesn't exist
mkdir -p "$TARGET_DIR"

# Download keys
curl -fsSL "$GOOGLE_CLOUD_SDK_KEY_URL" | gpg --dearmor -o "$TARGET_DIR/google-cloud-sdk.gpg"
curl -fsSL "$DOCKER_KEY_URL" | gpg --dearmor -o "$TARGET_DIR/docker.gpg"
curl -fsSL "$YARN_KEY_URL" | gpg --dearmor -o "$TARGET_DIR/yarn.gpg"
curl -fsSL "$GOOGLE_CHROME_KEY_URL" | gpg --dearmor -o "$TARGET_DIR/google-chrome.gpg"
curl -fsSL "$NODE_SOURCE_KEY_URL" | gpg --dearmor -o "$TARGET_DIR/nodesource.gpg"
curl -fsSL "$ADOPTIUM_KEY_URL" | gpg --dearmor -o "$TARGET_DIR/adoptium.gpg"
curl -fsSL "$NVIDIA_KEY_URL" | gpg --dearmor -o "$TARGET_DIR/nvidia.gpg"

echo "All keys downloaded successfully."
