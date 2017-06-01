#!/bin/sh

dir=$1

[ -z "$dir" ] && echo "Usage: $0 asset_and_program_dir" && exit 1
[ ! -d "$dir" ] && echo "Asset and program directory $dir does not exist" && exit 1

systemctl stop laitos
rm -rf /root/laitos

set -e
mkdir -p /root/laitos
cp -R "$dir"/* /root/laitos/
cp "$dir"/laitos.service /etc/systemd/system/

systemctl daemon-reload
systemctl enable laitos
systemctl restart laitos
