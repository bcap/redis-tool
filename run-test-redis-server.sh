#!/bin/bash

set -e 

DATA_DIR="$(readlink -f $(dirname $0)/.redis-data)"

mkdir -p $DATA_DIR

chmod 777 $DATA_DIR

docker run \
    --rm \
    --name redis \
    --network host \
    -v ${DATA_DIR}:/data \
    redis \
    bash -c "redis-server --save 1 1 --loglevel warning"