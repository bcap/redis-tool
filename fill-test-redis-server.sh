#!/bin/bash

set -e -u -o pipefail

function log() {
    echo "=> $(date) | $@" >&2
}

DATA_FILE=$(mktemp)
LOG_FILE=$(mktemp)

trap "rm $DATA_FILE $LOG_FILE" exit

log generating data file at $DATA_FILE

awk '
function ceil(x){
    y = int(x)
    return(x>y?y+1:y)
}

BEGIN {

    STRING_KEYS=1000000
    STRING_BATCH=100

    HASH_KEYS=10000
    HASH_VALUES=100
    HASH_BATCH=100

    LIST_KEYS=10000
    LIST_VALUES=100
    LIST_BATCH=100

    SET_KEYS=10000
    SET_VALUES=100
    SET_BATCH=100

    ZSET_KEYS=10000
    ZSET_VALUES=100
    ZSET_BATCH=100

    for (i=0; i < STRING_KEYS; i++) {
        if (i % STRING_BATCH == 0) {
            printf "\nMSET "
        }
        printf "test-string-%010d test-string-value-%010d ", i, i
    }
    printf "\n"

    for (i=0; i < HASH_KEYS; i++) {
        printf "DEL test-hash-%010d\n", i
        for (j=0; j < HASH_VALUES; j++) {
            if (j % HASH_BATCH == 0) {
                printf "\nHMSET test-hash-%010d ", i
            }
            printf "test-hash-%010d-%010d test-hash-value-%010d-%010d ", i, j, i, j
        }
        printf "\n"
    }

    for (i=0; i < LIST_KEYS; i++) {
        printf "DEL test-list-%010d\n", i
        for (j=0; j < LIST_VALUES; j++) {
            if (j % LIST_BATCH == 0) {
                printf "\nLPUSH test-list-%010d ", i
            }
            printf "test-list-value-%010d-%010d ", i, j
        }
        printf "\n"
    }

    for (i=0; i < SET_KEYS; i++) {
        printf "DEL test-set-%010d\n", i
        for (j=0; j < SET_VALUES; j++) {
            if (j % SET_BATCH == 0) {
                printf "\nSADD test-set-%010d ", i
            }
            printf "test-set-value-%010d-%010d ", i, j
        }
        printf "\n"
    }

    for (i=0; i < ZSET_KEYS; i++) {
        printf "DEL test-zset-%010d\n", i
        for (j=0; j < ZSET_VALUES; j++) {
            if (j % ZSET_BATCH == 0) {
                printf "\nZADD test-zset-%010d ", i
            }
            printf "%d test-zset-value-%010d-%010d ", j, i, j
        }
        printf "\n"
    }

}' | grep . > $DATA_FILE

log writing data to redis and logging to $LOG_FILE

# cat $DATA_FILE

pv $DATA_FILE | redis-cli -u redis://localhost:6379 > $LOG_FILE

log Commands Aggregated Results:

sort $LOG_FILE | uniq -c | sort -n | while read line; do log $line; done