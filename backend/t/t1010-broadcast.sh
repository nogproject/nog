#!/bin/bash
# vim: sw=4

test_description='
event broadcast
'

. ./lib.sh
trap cleanupFsods EXIT
test_expect_success 'start fsods' 'startFsods'

test_expect_success 'broadcast main and registry' '
    nogfsoctl events broadcast | grep EV_BC_FSO_MAIN_CHANGED &&
    nogfsoctl events broadcast | grep EV_BC_FSO_REGISTRY_CHANGED &&
    ! nogfsoctl events broadcast | grep EV_BC_FSO_REPO_CHANGED &&
    :
'

test_expect_success 'broadcast repo' '
    mkdir "d a t/foo" &&
    touch "d a t/foo/1" &&
    : &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/foo" &&
    : &&
    waitGrep 5s "Confirmed init shadow" stad.log &&
    waitGrep 5s "Confirmed init GitLab" stad.log &&
    : &&
    nogfsoctl events broadcast | grep EV_BC_FSO_REPO_CHANGED &&
    :
'

test_expect_success 'shutdown fsods' 'shutdownFsods'
test_done
