#!/usr/bin/env bash

[ "$(basename "$(pwd)")" != elasticbeanstalk ] && echo 'Please run the script from "elasticbeanstalk" directory.' && exit 1

archive=$1
procfile=$2
if [ -z "$archive" ] || [ -z "$procfile" ]; then
 echo "Usage: $0 archive-path procfile-path" && exit 1
fi
if [ ! -f "$archive" ] || [ ! -f "$procfile" ]; then
  echo "Failed to read archive or procfile" && exit 1
fi

set -e

pushd ../../
env GOCACHE=off CGO_ENABLED=0 GOOS=linux go build
popd

rm -rf bundle/
mkdir bundle/
cp "$archive" "$procfile" bundle/
cp -R ../../laitos ./.ebextensions bundle/
chmod -R 755 bundle/

pushd bundle
zip -r bundle.zip ./
mv bundle.zip ../
popd

rm -rf ./bundle

echo "bundle.zip is ready!"
