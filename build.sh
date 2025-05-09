#!/bin/bash
# Copyright 2024 Google LLC
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

#
# Build script that will get the module dependencies and build Linux and
# Windows binaries. The google_cloud_workload_agent binary will be built into
# the buildoutput/ dir.
#

#
# Note: use the following to get the latest sapagent version for the shared
# libraries:
#
# go list -m -json github.com/GoogleCloudPlatform/sapagent@main
#
# Then update go.mod with the version from the output.
#
# If the build is failing because of dependencies then update the go.mod and
# go.sum with the latest versions from the buildoutput/ directory.
#

set -exu

echo "Starting the build process for the Workload Agent Platform..."

echo "**************  Getting go 1.24.2"
curl -sLOS https://go.dev/dl/go1.24.2.linux-amd64.tar.gz
chmod -fR u+rwx /tmp/wap || :
rm -fr /tmp/wap
mkdir -p /tmp/wap
tar -C /tmp/wap -xzf go1.24.2.linux-amd64.tar.gz

export GOROOT=/tmp/wap/go
export GOPATH=/tmp/wap/gopath
mkdir -p "${GOPATH}"
mkdir -p $GOROOT/.cache
mkdir -p $GOROOT/pkg/mod
export GOMODCACHE=$GOROOT/pkg/mod
export GOCACHE=$GOROOT/.cache
export GOBIN=$GOROOT/bin

PATH=${GOBIN}:${GOROOT}/packages/bin:$PATH

echo "**************  Getting unzip 5.51"
curl -sLOS https://oss.oracle.com/el4/unzip/unzip.tar
tar -C /tmp/wap -xf unzip.tar

echo "**************  Getting protoc 28.2"
pb_rel="https://github.com/protocolbuffers/protobuf/releases"
pb_dest="/tmp/wap/protobuf"
curl -sLOS ${pb_rel}/download/v28.2/protoc-28.2-linux-x86_64.zip
rm -fr "${pb_dest}"
mkdir -p "${pb_dest}"
/tmp/wap/unzip -q protoc-28.2-linux-x86_64.zip -d "${pb_dest}"

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

echo "**************  Compiling protobufs"
protoc --go_opt=paths=source_relative integration/common/protos/*.proto --go_out=.
protoc --go_opt=paths=source_relative integration/example/protos/*.proto --go_out=.
protoc --go_opt=paths=source_relative sharedprotos/**/*.proto --go_out=.

echo "**************  Builidiing the example agent"
pushd integration/example

mkdir -p buildoutput
echo "**************  Generating the latest go.mod and go.sum dependencies"
cp go.mod go.mod.orig
cp go.sum go.sum.orig
go clean -modcache
go mod tidy
mv go.mod buildoutput/go.mod.latest
mv go.sum buildoutput/go.sum.latest
mv go.mod.orig go.mod
mv go.sum.orig go.sum

echo "**************  Getting the repo module dependencies using go mod vendor"
go clean -modcache
go mod vendor

echo "**************  Running all tests"
go test ./...

pushd cmd
echo "**************  Building Linux binary"
env GOOS=linux GOARCH=amd64 go build -mod=vendor -v -o ../buildoutput/google_cloud_example_agent

echo "**************  Building Windows binary"
env GOOS=windows GOARCH=amd64 go build -mod=vendor -v -o ../buildoutput/google_cloud_example_agent.exe
popd # cmd
popd # integration/example
echo "**************  Finished building the Workload Agent, the binaries and latest go.mod/go.sum are available in the buildoutput directory"
