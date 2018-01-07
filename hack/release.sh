#!/bin/bash
set -xeou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT="$GOPATH/src/github.com/kubedb/postgres"

export APPSCODE_ENV=prod

pushd $REPO_ROOT

rm -rf dist

./hack/docker/pg-operator/setup.sh
./hack/docker/pg-operator/setup.sh release

rm dist/.tag

popd
