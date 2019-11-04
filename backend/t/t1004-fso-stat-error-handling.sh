#!/bin/bash
# vim: sw=4

test_description='
fso stat error handling
'

. ./lib.sh

trap cleanupFsods EXIT
test_expect_success 'start fsods' 'startFsods'

test_expect_success 'init repo' '
    mkdir "d a t/foo" &&
    touch "d a t/foo/1" &&
    : &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/foo" &&
    : &&
    waitGrep 5s "Confirmed init shadow" stad.log &&
    waitGrep 5s "Confirmed init GitLab" stad.log &&
    repoId=$(getRepoId exreg "/'${fsoExampleNs}'/d a t/foo") &&
    true
'

# `--jwt-auth=no`, because the Meteor app does not know the repo.
test_expect_success 'stat error is stored as repo error' '
    touch "d a t/foo/2" &&
    chmod a-rwx "d a t/foo/2" &&
    : &&
    nogfsoctl stat --jwt-auth=no --author="A U Thor <author@example.com>" "${repoId}" &&
    : &&
    waitGrep 5s "error.*git-fso stat failed" stad.log &&
    waitGrep 5s "Stored repo error" stad.log &&
    nogfsoctl get repo --jwt-auth=no "${repoId}" | grep "\"error\".*" &&
    true
'

test_expect_success 'shutdown fsods' 'shutdownFsods'
test_done
