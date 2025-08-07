#!/bin/sh

set -ex

ARCH=$(uname -m)
case $ARCH in
    x86_64)
        NODE_ARCH="x64"
        ;;
    aarch64|arm64)
        NODE_ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

cd /tmp
curl -LO https://nodejs.org/dist/v20.13.1/node-v20.13.1-linux-${NODE_ARCH}.tar.xz
tar -xJf node-v20.13.1-linux-${NODE_ARCH}.tar.xz
mv node-v20.13.1-linux-${NODE_ARCH} /usr/local/nodejs
rm node-v20.13.1-linux-${NODE_ARCH}.tar.xz

ln -s /usr/local/nodejs/bin/node /usr/bin/node
ln -s /usr/local/nodejs/bin/npm /usr/bin/npm
ln -s /usr/local/nodejs/bin/npx /usr/bin/npx
ln -s /usr/local/nodejs/bin/pnpm /usr/bin/pnpm

npm install -g pnpm@8.15.8 @microsoft/rush@5.147.1
ln -s /usr/local/nodejs/bin/rush /usr/bin/rush
