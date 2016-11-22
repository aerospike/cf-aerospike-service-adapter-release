#!/bin/bash
#set -xv

function realpath() {
  [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}

SCRIPT=$(realpath $0)
SCRIPT_DIR=$(dirname $SCRIPT)
ROOT_DIR=$SCRIPT_DIR/..

if [ "$#" -gt 1 ]; then
  echo "Usage: [<version>]"
  echo " 1st arg (optional): version for the release" 
  echo ""
  exit -1
fi

RELEASE_DIR=$(realpath $ROOT_DIR)

source $SCRIPT_DIR/version
if [ "$#" -gt 0 ]; then
  SERVICE_ADAPTER_RELEASE_VERSION="$1"
  printf "SERVICE_ADAPTER_RELEASE_VERSION=%s\n" $SERVICE_ADAPTER_RELEASE_VERSION > $SCRIPT_DIR/version
else
  # Increment current version and save it back out
  perl -i -pe 's/SERVICE_ADAPTER_RELEASE_VERSION=\d+\.\d+\.\K(\d+)/ $1+1 /e' $SCRIPT_DIR/version
  source $SCRIPT_DIR/version
fi

PRODUCT_NAME=aerospike-service-adapter
RELEASE_NAME=aerospike-service-adapter-release

cd $RELEASE_DIR

# Uncomment if the jobs have to be generated each time
rm -rf .dev_releases *releases .*builds  ~/.bosh/cache
bosh -n create release --name $RELEASE_NAME \
     --version $SERVICE_ADAPTER_RELEASE_VERSION --force --with-tarball