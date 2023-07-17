#!/bin/bash

set -e 

cd $(dirname $0)

DATA_DIR="data"

mkdir -p $DATA_DIR

chmod 755 $DATA_DIR

set -x

docker build -f ./Dockerfile.redis -t redis-tool/test-redis .

docker run \
    --rm \
    --name redis \
    --network host \
    -v $(readlink -f ${DATA_DIR}):/data \
    redis-tool/test-redis