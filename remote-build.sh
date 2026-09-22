#!/usr/bin/env bash
set -euo pipefail

REMOTE_HOST=""
REPO_DIR="$(pwd)"
REMOTE_DIR="/tmp/netmaker-build"

IMAGE_NAME="abhi9686/netmaker"
IMAGE_TAG="NM-366"
DOCKERFILE="Dockerfile"

PLATFORM=""
BUILD_CMD=""
NO_CACHE=false
PUSH=false

usage() {
  echo "Usage:"
  echo "  $0 --host user@server [options]"
  echo ""
  echo "Options:"
  echo "  --repo /path/to/repo        Local repo path. Defaults to current directory"
  echo "  --dir /remote/path          Remote build path. Defaults to /tmp/netmaker-build"
  echo "  --image repo/name           Docker image name. Defaults to gravitl/netmaker"
  echo "  --tag tag                   Docker image tag. Defaults to dev"
  echo "  --dockerfile Dockerfile     Dockerfile to use. Defaults to Dockerfile"
  echo "  --build-cmd \"cmd\"           Command to run on remote before docker build"
  echo "  --platform linux/amd64      Optional Docker build platform"
  echo "  --no-cache                  Build without Docker cache"
  echo "  --push                      Push image after build"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) REMOTE_HOST="$2"; shift 2 ;;
    --repo) REPO_DIR="$(realpath "$2")"; shift 2 ;;
    --dir) REMOTE_DIR="$2"; shift 2 ;;
    --image) IMAGE_NAME="$2"; shift 2 ;;
    --tag) IMAGE_TAG="$2"; shift 2 ;;
    --dockerfile) DOCKERFILE="$2"; shift 2 ;;
    --build-cmd) BUILD_CMD="$2"; shift 2 ;;
    --platform) PLATFORM="$2"; shift 2 ;;
    --no-cache) NO_CACHE=true; shift ;;
    --push) PUSH=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1"; usage; exit 1 ;;
  esac
done

[[ -n "$REMOTE_HOST" ]] || { echo "Missing required option: --host"; usage; exit 1; }
[[ -d "$REPO_DIR" ]] || { echo "Repository path not found: $REPO_DIR"; exit 1; }

BUILD_ARGS=""
[[ -n "$PLATFORM" ]] && BUILD_ARGS+=" --platform $PLATFORM"
[[ "$NO_CACHE" == true ]] && BUILD_ARGS+=" --no-cache"

echo "==> Repo: $REPO_DIR"
echo "==> Remote: $REMOTE_HOST:$REMOTE_DIR"
echo "==> Image: $IMAGE_NAME:$IMAGE_TAG"
echo "==> Dockerfile: $DOCKERFILE"

ssh "$REMOTE_HOST" "mkdir -p '$REMOTE_DIR'"

echo "==> Syncing code to remote machine..."
rsync -az --delete \
  --exclude ".git" \
  --exclude "node_modules" \
  --exclude "dist" \
  --exclude "build" \
  --exclude ".env" \
  --exclude "tmp" \
  "$REPO_DIR"/ "$REMOTE_HOST:$REMOTE_DIR/"

echo "==> Building Docker image on remote machine..."
ssh "$REMOTE_HOST" bash -s <<EOF
set -euo pipefail

cd "$REMOTE_DIR"

if [[ -n "$BUILD_CMD" ]]; then
  echo "==> Running build command..."
  eval "$BUILD_CMD"
fi

docker build $BUILD_ARGS -f "$DOCKERFILE" -t "$IMAGE_NAME:$IMAGE_TAG" .
EOF

if [[ "$PUSH" == true ]]; then
  echo "==> Pushing Docker image..."
  ssh "$REMOTE_HOST" bash -s <<EOF
set -euo pipefail
docker push "$IMAGE_NAME:$IMAGE_TAG"
EOF
fi

echo "==> Done"
echo "Built image: $IMAGE_NAME:$IMAGE_TAG"
