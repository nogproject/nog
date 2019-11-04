#!/bin/bash
set -o nounset -o errexit -o pipefail -o noglob

tarx() {
    gtar "$@"
}

mkdir -p intermediate

tarx --dereference -cf - nogd.py | tarx -C intermediate -xvf -

find intermediate -type d -exec chmod 0755 '{}' ';'
find intermediate -type f -exec chmod 0644 '{}' ';'
chmod 0755 nogreplicad
