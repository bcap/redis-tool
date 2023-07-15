#!/bin/bash

set -e 

KEYS=1000000
BATCH=100

seq $KEYS | 
    awk '{printf "test-key-%010d test-value-%010d\n", $1, $1}' | 
    xargs -n $(( BATCH * 2 )) echo MSET |
    pv -l -s $(( KEYS / BATCH )) | 
    redis-cli >/dev/null