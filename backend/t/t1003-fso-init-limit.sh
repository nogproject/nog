#!/bin/bash
# vim: sw=4

test_description='
fso files and data realdir init limits
'

. ./lib.sh

trap cleanupFsods EXIT
test_expect_success 'start fsods' '
startFsodsStadOpts --init-limit-max-files=10 --init-limit-max-size=2k
'

# Directories are counted: 1 dir + 10 files = 11 > limit of 10.
test_expect_success 'repo init rejects too many files' '
    mkdir "d a t/foo" &&
    for i in {1..10}; do
        touch "d a t/foo/$i"
    done \
    &&
    ! nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/foo" 2>/dev/null \
    &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/foo" 2>&1 \
    | grep "err.*more than 10 files" \
    &&
    true
'

test_expect_success 'repo init accepts number of files below limit' '
    rm "d a t/foo/10" &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/foo" \
    && \
    true
'

test_expect_success 'repo init rejects too much data' '
    mkdir "d a t/bar" &&
    dd if=/dev/zero of="d a t/bar/data" bs=1k count=3 \
    &&
    ! nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/bar" 2>/dev/null \
    &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/bar" 2>&1 \
    | grep "err.*contains more than.*bytes" \
    &&
    true
'

test_expect_success 'repo init accepts data size below limit' '
    echo "" >"d a t/bar/data" && \
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/bar" \
    && \
    true
'

test_expect_success 'shutdown fsods' 'shutdownFsods'
test_done
