#!/bin/bash
# vim: sw=4

test_description='
Basic fso with GitLab; see also t1006
'

. ./lib.sh

# Use separate database for each test run.  Name based on Unix nanos.
ts=$(nanos)
mongodb="%2Fmongo%2Frun%2Fmongodb-27017.sock/nogfsoreg-t${ts}"
example="example-t${ts}"

cleanup() {
    (
        pgrep nogfsoregd || true
        pgrep nogfsostad || true
    ) \
    | xargs --verbose --no-run-if-empty kill -s KILL
}

trap cleanup EXIT

test_expect_success 'nogfsoregd start' '
    nogfsoregd --log=mu --mongodb="'${mongodb}'" 2>&1 | tee regd.log &
    waitPort 5s 7550 &&
    waitPort 5s 7551 &&
    true
'

test_expect_success 'init registry' '
    nogfsoctl init registry --no-vid exreg &&
    nogfsoctl init registry --no-vid exreg &&
    nogfsoctl get registries | grep "^- .*name.*exreg" &&
    true
'

test_expect_success 'nogfsostad start' '
    mkdir shadow &&
    nogfsostad --log=mu \
        --shadow-root="'"$PWD"'/shadow" \
        --host=files.example.com \
        --prefix="/'${example}'/f i l e s" \
        --prefix="/'${example}'/data" \
        --gitlab-token=/etc/gitlab/root.token \
        exreg 2>&1 | tee stad.log &
    waitGrep 5s "GRPC listening disabled" stad.log &&
    true
'

test_expect_success 'init roots' '
    nogfsoctl init root exreg --no-vid \
        --host=files.example.com \
        --gitlab-namespace=localhost/root \
        "/'${example}'/f i l e s" \
        "'"$PWD"'/f i l e s" &&
    nogfsoctl init root exreg --no-vid \
        --host=files.example.com \
        --gitlab-namespace=localhost/root \
        "/'${example}'/f i l e s" \
        "'"$PWD"'/f i l e s" &&
    nogfsoctl get roots exreg | grep "'${example}'" &&
    true
'

test_expect_success 'remove root' '
    nogfsoctl init root exreg --no-vid \
        --host=files.example.com \
        "/'${example}'/tmp" \
        "'"$PWD"'/tmp" &&
    nogfsoctl get roots exreg | grep "'${example}'/tmp" &&
    nogfsoctl remove root exreg --no-vid "/'${example}'/tmp" &&
    ! nogfsoctl get roots exreg | grep "'${example}'/tmp" &&
    true
'

test_expect_success '`nogfsoctl init repo` rejects invalid path' '
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid /invalid/path 2>&1 \
    | grep "fatal.*unknown root" &&
    true
'

test_expect_success 'init basic file repo' '
    mkdir "f i l e s" "f i l e s/f o o" && touch "f i l e s/f o o/b a r" &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${example}'/f i l e s/f o o" &&
    nogfsoctl get repos exreg | grep "'${example}'/f i l e s/f o o" &&
    waitGrep 5s "Confirmed init shadow" stad.log &&
    waitGrep 5s "Confirmed init GitLab" stad.log &&
    true
'

test_expect_success 'get repo uuid' '
    repoId=$(getRepoId exreg "/'${example}'/f i l e s/f o o")
'

test_expect_success 'registry info' '
    nogfsoctl info exreg | grep "^numRoots: 1" &&
    nogfsoctl info exreg | grep "^numRepos: 1" &&
    nogfsoctl get roots exreg | grep "'${example}'" &&
    nogfsoctl get repos exreg | grep "'${example}'/f i l e s/f o o" &&
    true
'

test_expect_success 'registry events' '
    [ $(nogfsoctl events registry exreg | wc -l) = 6 ] &&
    nogfsoctl events registry exreg | grep FSO_REGISTRY_ADDED &&
    nogfsoctl events registry exreg | grep FSO_ROOT_ADDED &&
    nogfsoctl events registry exreg | grep FSO_ROOT_REMOVED &&
    nogfsoctl events registry exreg | grep FSO_REPO_ACCEPTED &&
    nogfsoctl events registry exreg | grep FSO_REPO_ADDED &&
    true
'

# The shadow path is long.  Go yaml.v2 would wrap it.
# `get repo --jwt-auth=no`, because the Meteor app does not know the repo.
test_expect_success 'get repo outputs JSON with unwrapped lines' '
    nogfsoctl get repo --jwt-auth=no "${repoId}" \
    | grep "\"shadow\": *\".*/f i l e s/f o o/${repoId}.fso\""
'

# `events repo --jwt-auth=no`, because the Meteor app does not know the repo.
test_expect_success 'repo events' '
    [ $(nogfsoctl events repo --jwt-auth=no "${repoId}" | wc -l) = 3 ] &&
    nogfsoctl events repo --jwt-auth=no "${repoId}" | grep FSO_REPO_INIT_STARTED &&
    nogfsoctl events repo --jwt-auth=no "${repoId}" | grep FSO_SHADOW_REPO_CREATED &&
    nogfsoctl events repo --jwt-auth=no "${repoId}" | grep FSO_GIT_REPO_CREATED &&
    true
'

# `--jwt-auth=no`, because the Meteor app does not know the repo.
test_expect_success 'stat' '
    nogfsoctl stat --jwt-auth=no --author="A U Thor <author@example.com>" "${repoId}" &&
    waitGrep 10s "Pushed git.*/${repoId}" stad.log &&
    true
'

# `--jwt-auth=no`, because the Meteor app does not know the repo.
test_expect_success 'sha' '
    logRotate stad.log &&
    nogfsoctl sha --jwt-auth=no --author="A U Thor <author@example.com>" "${repoId}" &&
    waitGrep 10s "Pushed git.*/${repoId}" stad.log &&
    true
'

fakeFixedUuid="$(uuid)"

test_expect_success 'init repo with given UUID' '
    logRotate stad.log &&
    mkdir "f i l e s/foo2" && touch "f i l e s/foo2/bar" &&
    nogfsoctl init repo --uuid='${fakeFixedUuid}' \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${example}'/f i l e s/foo2" &&
    nogfsoctl get repos exreg | grep "'${fakeFixedUuid}.*${example}'/f i l e s/foo2" &&
    true
'
# Keep together with previous test.
test_expect_success 'init repo stores UUID in `.git/fso/uuid`' '
    waitGrep 5s "Confirmed init shadow" stad.log &&
    grep "^'${fakeFixedUuid}'$" "'"$PWD/shadow/$PWD/f i l e s/foo2/${fakeFixedUuid}.fso/.git/fso/uuid"'" &&
    true
'

test_expect_success 'nogfsostad shutdown' '
    kill -s TERM $(pgrep nogfsostad) &&
    waitGrep 5s "Completed graceful shutdown" stad.log &&
    true
'

test_expect_success 'nogfsoregd shutdown' '
    kill -s TERM $(pgrep nogfsoregd) &&
    waitGrep 5s "Completed graceful shutdown" regd.log &&
    true
'

test_done
