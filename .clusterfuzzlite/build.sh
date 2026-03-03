#!/bin/bash -eu
################################################################################
# ClusterFuzzLite build script for AgentCert
# Adapted from chaoscenter/fuzz_build.sh
################################################################################
export GO_MOD_PATHS_MAPPING=( "graphql/server" "authentication" "subscriber" )

cd chaoscenter
export rootDir=$(pwd)

for dir in "${GO_MOD_PATHS_MAPPING[@]}"; do
    cd ${dir} && go mod download
    go install github.com/AdamKorcz/go-118-fuzz-build@latest
    go get github.com/AdamKorcz/go-118-fuzz-build/testing
    fuzz_files=($(find "$(pwd)" -type f -name '*_fuzz_test.go'))
    for file in "${fuzz_files[@]}"; do
        pkg=$(grep -m 1 '^package' "$file" | awk '{print $2}')
        package_path=$(dirname "${file%$pkg}")
        functionList=($(grep -o 'func Fuzz[A-Za-z0-9_]*' ${file} | awk '{print $2}'))
        for i in "${functionList[@]}"
        do
            compile_native_go_fuzzer ${package_path} ${i} ${i}
        done
    done
    cd ${rootDir}
done
