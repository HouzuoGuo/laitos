#!/bin/sh

[ "$(basename $(pwd))" != elasticbeanstalk ] && echo 'Please run the script from "elasticbeanstalk" directory.' && exit 1

config_dir=$1
[ -z "$config_dir" ] && echo "Usage: $0 asset_and_config_dir" && exit 1
[ ! -d "$config_dir" ] && echo "Asset and configuration directory $config_dir does not exist" && exit 1

set -e

pushd ../../
env CGO_ENABLED=0 go build
popd

rm -rf bundle/
cp -R "$config_dir" bundle/
cp -R ../../laitos ./Procfile ./.ebextensions/ ../phantom* ../laitos.service ../deploy_to_root_home.sh bundle/
chmod -R 755 bundle/

pushd bundle
zip -r bundle.zip ./
mv bundle.zip ../
popd

rm -rf bundle

echo "bundle.zip is ready!"
