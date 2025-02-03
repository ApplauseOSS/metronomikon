#!/bin/bash

K3S_IMAGE_TAG="v1.31.4-k3s1"
K3D_VERSION=5.8.1
CLUSTER_NAME=metronomikon-ci

BASEDIR=$(cd $(dirname $0)/..; pwd -P)

set -e

cd ${BASEDIR}

# Install k3d
k3d --version >/dev/null || \
	curl -s https://raw.githubusercontent.com/rancher/k3d/main/install.sh | TAG=v${K3D_VERSION} bash

# Create k3d cluster
k3d cluster delete ${CLUSTER_NAME} || true
k3d cluster create \
	${CLUSTER_NAME} \
	--image rancher/k3s:${K3S_IMAGE_TAG} \
	--port 8080:80@loadbalancer:*:direct

# Build
make test
make build
make image

# Import image
k3d image import -c ${CLUSTER_NAME} applause/metronomikon:latest

echo "Waiting for traefik to be ready"
while ! curl -s 127.0.0.1:8080/ping > /dev/null; do
	sleep 1
done

# Create venv and install test dependencies
python3 -m venv test/.virtualenv
source test/.virtualenv/bin/activate
pip install -r test/requirements.txt

# Run functional tests
make functest
