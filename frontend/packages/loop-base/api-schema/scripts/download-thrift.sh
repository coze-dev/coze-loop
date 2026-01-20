#!/bin/bash

# [AI-Generated Code Start]
# Download idl/thrift directory from coze-loop repository

set -e

REPO_URL="https://github.com/coze-dev/coze-loop.git"
DEFAULT_BRANCH="main"
CACHE_DIR="node_modules/.cache"
TARGET_DIR="coze-loop-idl"

# Get script directory and repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if current repository is coze-dev/coze-loop
is_coze_loop_repo() {
  local remote_url
  remote_url=$(git remote get-url origin 2>/dev/null || echo "")
  if [[ "$remote_url" == *"coze-dev/coze-loop"* ]]; then
    return 0
  fi
  return 1
}

# Copy idl/thrift from local coze-loop repository
copy_from_local() {
  echo "Detected coze-loop repository, copying from local idl/thrift..."
  local REPO_ROOT="$(cd "$SCRIPT_DIR/../../../../.." && pwd)"
  local LOCAL_IDL_DIR="$REPO_ROOT/idl/thrift"

  if [ -d "$LOCAL_IDL_DIR" ]; then
    mkdir -p "$OUTPUT_DIR"
    cp -r "$LOCAL_IDL_DIR" "$OUTPUT_DIR/"
    echo "Done: $OUTPUT_DIR/thrift (copied from local repository)"
  else
    echo "Error: Local idl/thrift directory not found at $LOCAL_IDL_DIR"
    exit 1
  fi
}

# Download idl/thrift from remote coze-loop repository
download_from_remote() {
  # Use default branch if not specified or empty
  if [ -z "$BRANCH" ]; then
    BRANCH="$DEFAULT_BRANCH"
  fi

  echo "Downloading idl/thrift from coze-loop (branch: $BRANCH)..."

  # Create temp directory for sparse checkout
  local TEMP_DIR
  TEMP_DIR=$(mktemp -d)
  trap "rm -rf $TEMP_DIR" EXIT

  echo "Initializing sparse checkout..."

  cd "$TEMP_DIR"

  # Initialize empty repository
  git init -q

  # Add remote
  git remote add origin "$REPO_URL"

  # Configure sparse checkout
  git config core.sparseCheckout true

  # Only checkout idl/thrift directory
  echo "idl/thrift" > .git/info/sparse-checkout

  # Fetch specified branch (shallow clone to reduce download size)
  echo "Fetching branch $BRANCH..."
  git fetch --depth=1 origin "$BRANCH"
  git checkout FETCH_HEAD -q

  # Return to original directory
  cd - > /dev/null

  # Copy idl/thrift to cache
  if [ -d "$TEMP_DIR/idl/thrift" ]; then
    mkdir -p "$OUTPUT_DIR"
    cp -r "$TEMP_DIR/idl/thrift" "$OUTPUT_DIR/"
    echo "Done: $OUTPUT_DIR/thrift"
  else
    echo "Error: idl/thrift directory not found"
    exit 1
  fi
}

# Parse command line arguments
BRANCH=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --branch)
      BRANCH="$2"
      shift 2
      ;;
    --branch=*)
      BRANCH="${1#*=}"
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--branch <branch_name>]"
      echo ""
      echo "Options:"
      echo "  --branch    Branch name to download (default: main)"
      echo "  -h, --help  Show help"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1"
      exit 1
      ;;
  esac
done

# Create cache directory
mkdir -p "$CACHE_DIR"

# Output path
OUTPUT_DIR="$CACHE_DIR/$TARGET_DIR"

# Remove existing directory if exists
if [ -d "$OUTPUT_DIR" ]; then
  echo "Cleaning existing directory: $OUTPUT_DIR"
  rm -rf "$OUTPUT_DIR"
fi

# Check if we are in the coze-loop repository and execute accordingly
if is_coze_loop_repo; then
  copy_from_local
else
  download_from_remote
fi

echo "Complete!"
# [AI-Generated Code End]
