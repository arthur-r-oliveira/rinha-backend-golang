#!/bin/bash

# Exit on error
set -e

# --- Configuration ---
QUAY_REPO="quay.io/rhn_support_arolivei"
HAPROXY_IMAGE_NAME="rinha-haproxy-ubi"
APP_IMAGE_NAME="rinha-de-backend-2025-golang-ubi"
TAG="latest"

# --- Build and Push HAProxy Image ---
echo "Building HAProxy UBI image..."
podman build -f haproxy/Dockerfile.ubi -t "$QUAY_REPO/$HAPROXY_IMAGE_NAME:$TAG" .
echo "Pushing HAProxy UBI image to $QUAY_REPO..."
podman push "$QUAY_REPO/$HAPROXY_IMAGE_NAME:$TAG"

# --- Build and Push Go App Image ---
echo "Building Go App UBI image..."
podman build -f api/Dockerfile.ubi -t "$QUAY_REPO/$APP_IMAGE_NAME:$TAG" ./api
echo "Pushing Go App UBI image to $QUAY_REPO..."
podman push "$QUAY_REPO/$APP_IMAGE_NAME:$TAG"

echo "
✅ All images built and pushed successfully!

HAProxy Image: $QUAY_REPO/$HAPROXY_IMAGE_NAME:$TAG
Go App Image:  $QUAY_REPO/$APP_IMAGE_NAME:$TAG

Update your docker-compose.yml to use these new images.
"