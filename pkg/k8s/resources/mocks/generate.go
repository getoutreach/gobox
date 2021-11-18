package mocks

//go:generate /usr/bin/env bash -c "pushd ../../../.. >/dev/null 2>&1 && ./scripts/shell-wrapper.sh gobin.sh sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0 object paths=./pkg/k8s/resources/mocks; popd >/dev/null 2>&1"
