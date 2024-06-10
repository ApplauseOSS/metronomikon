#!/bin/bash

set -o pipefail

TEST_NAMESPACE=metronomikon-test

BASEDIR=$(readlink -f $(dirname $0))
TMPDIR=$(mktemp -d)

die() {
	echo "$1"
	exit 1
}

test_kubectl() {
	echo "Testing kubectl..."
	if ! kubectl get nodes >/dev/null; then
		echo "kubectl does not appear to be functional for basic operations"
		echo "You will need to install/configure kubectl before running the functional tests"
		exit 1
	fi
}

populate_test_data() {
	echo "Populating test data..."
	for i in $(ls -1 ${BASEDIR}/data/*.yaml | sort); do
		kubectl apply -f $i || die "failed to apply test data"
	done
}

deploy_metronomikon() {
	cat ${BASEDIR}/../example/k8s/kube-manifest.yaml | sed -e "s/kube-system/${TEST_NAMESPACE}/" > ${TMPDIR}/deploy.yaml
	kubectl apply -f ${TMPDIR}/deploy.yaml || die "failed to deploy metronomikon"
	kubectl rollout status --timeout 30s --watch -n ${TEST_NAMESPACE} deployment/metronomikon || die "failed to deploy metronomikon"
	kubectl apply -f ${BASEDIR}/../example/k8s/metronomikon-ingress.yaml
}

cleanup_test_data() {
	echo "Cleaning up test data..."
	for i in $(ls -1 ${BASEDIR}/data/*.yaml | sort -r); do
		kubectl delete -f $i
	done
}

run_tests() {
	cd ${BASEDIR}
	pip list
	behave
}

main() {
	if [[ ! TMPDIR ]]; then
		die "failed to create temp dir"
	fi

	trap 'cleanup_test_data; rm -rf $TMPDIR' EXIT

	test_kubectl
	populate_test_data
	sleep 5
	deploy_metronomikon
	sleep 5
	run_tests
}

main $*
