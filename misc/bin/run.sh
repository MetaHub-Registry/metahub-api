#!/bin/bash

IMG=$1
shift

if [ -z "$IMG" ]; then
    echo "Usage: $0 <image> [args]"
    exit 1
fi

if [[ -z $1 ]];then
  echo ">> docker run  -ti --rm --pull=always ${MH_URL}/${IMG} 2>/dev/null"
  docker run  -ti --rm  --pull=always ${MH_URL}/${IMG} 2>/dev/null
else 
    echo ">> docker run  -ti --rm --pull=always ${MH_URL}/${IMG} $@ "
    docker run  -ti --rm --pull=always ${MH_URL}/${IMG} $@
fi