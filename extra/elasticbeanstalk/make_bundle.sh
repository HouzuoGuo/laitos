#!/bin/sh

[ "$(basename $(pwd))" != elasticbeanstalk ] && echo 'Please run the script from "elasticbeanstalk" directory.' && exit 1

archive=$1
procfile=$2
[ -z "$archive" -o -z "$procfile" ] && echo "Usage: $0 archive-path procfile-path" && exit 1
[ ! -f "$archive" -o ! -f "$procfile" ] && echo "Failed to read archive or procfile" && exit 1

set -e

pushd ../../
env CGO_ENABLED=0 go build
popd

rm -rf bundle/
mkdir bundle/
cp "$archive" "$procfile" bundle/
cp -R ../../laitos ./.ebextensions/ bundle/
chmod -R 755 bundle/

pushd bundle
zip -r bundle.zip ./
mv bundle.zip ../
popd

rm -rf bundle

echo "bundle.zip is ready!"
