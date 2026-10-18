#!/usr/bin/env bash
# Rebuild the Linux patch engine against the supported glibc 2.35 baseline.
set -euo pipefail
cd "$(dirname "$0")/.."
image=ubuntu:22.04@sha256:0a9887931b0e38af2e3487ed39e29436b0535c4f3f52e57745baaf79d1898956
commit=7508fd2a823443b1f0173ca361620f21d62a7d37 # upstream release3_1_apl (Apache-2.0)
source_sha256=e5aa66a79eab43e85597f54a15dce699488a301535afd264e30432cc076ad218
mkdir -p build
output=$(mktemp -d "$PWD/build/xdelta.XXXXXX")
trap 'rm -rf "$output"' EXIT

docker run --rm -i --platform linux/amd64 -v "$output:/output" "$image" \
  bash -euo pipefail -s -- "$commit" "$source_sha256" <<'BUILD'
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends build-essential autoconf automake libtool liblzma-dev ca-certificates curl
cd /tmp
curl --fail --location --retry 3 -o source.tar.gz "https://codeload.github.com/jmacd/xdelta/tar.gz/$1"
echo "$2  source.tar.gz" | sha256sum --check --status
tar -xzf source.tar.gz
cd "xdelta-$1/xdelta3"
autoreconf -fi
./configure --with-liblzma \
  CFLAGS='-O2 -std=gnu11 -include assert.h -fPIE -fstack-protector-strong -D_FORTIFY_SOURCE=2' \
  LDFLAGS='-pie -Wl,-z,relro,-z,now'
make -j2 xdelta3
strip --strip-unneeded xdelta3
./xdelta3 -V
cp xdelta3 /output/xdelta3_linux
BUILD

install -m 755 "$output/xdelta3_linux" internal/assets/xdelta3_linux
sha256sum internal/assets/xdelta3_linux
