#!/bin/bash
# vim: sw=4

cd "$(dirname "$BASH_SOURCE[0]")"
err=
for t in t????-*.sh; do
    echo
    echo "# $t"
    ./$t "$@" || err=t
done

echo
if test $err; then
    echo "FAIL (some tests failed; see errors above)"
    exit 1
else
    echo "OK (all tests passed)"
fi
