#!/bin/bash
# vim: sw=4

test_description='
fso handles git in realdirs
'

. ./lib.sh

trap cleanupFsods EXIT
test_expect_success 'start fsods' 'startFsods'

# `--jwt-auth=no`, because the Meteor app does not know the repo.
test_expect_success 'track git repo' '
    mkdir "d a t/gitrepo" &&
    touch "d a t/gitrepo/f o o" && (
        cd "d a t/gitrepo" &&
        git init &&
        git config user.name "A U Thor" &&
        git config user.email "author@example.org" &&
        git add . &&
        git commit -m init
    ) &&
    : &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/gitrepo" &&
    nogfsoctl get repos exreg | grep "'${fsoExampleNs}'/d a t/gitrepo" &&
    waitGrep 5s "Confirmed init shadow" stad.log &&
    waitGrep 5s "Confirmed init GitLab" stad.log &&
    repoId=$(getRepoId exreg "/'${fsoExampleNs}'/d a t/gitrepo") &&
    : &&
    nogfsoctl stat --jwt-auth=no --author="A U Thor <author@example.com>" "${repoId}" &&
    waitGrep 10s "Pushed git.*/${repoId}" stad.log &&
    : &&
    logRotate stad.log &&
    nogfsoctl sha --jwt-auth=no --author="A U Thor <author@example.com>" "${repoId}" &&
    waitGrep 10s "Pushed git.*/${repoId}" stad.log &&
    : &&
    true
'

test_expect_success 'shadow git contains git stat blob' '
    shadowGit="$(find shadow -type d | grep "gitrepo/[0-9a-f-]*.fso/.git$")" &&
    git -C "${shadowGit}" show "master-stat:.nogtree" \
    | egrep "^git: \"[0-9a-f]{40}\"$" &&
    true
'

# `--jwt-auth=no`, because the Meteor app does not know the repo.
test_expect_success 'track git repo with submodules' '
    mkdir "d a t/gitrepoSub" &&
    touch "d a t/gitrepoSub/f o o" && (
        cd "d a t/gitrepoSub" &&
        git init &&
        git config user.name "A U Thor" &&
        git config user.email "author@example.org" &&
        git add . &&
        git commit -m init &&
        mkdir "s u b" && (
            cd "s u b" &&
            git init &&
            git config user.name "A U Thor" &&
            git config user.email "author@example.org" &&
            touch subdat &&
            git add . &&
            git commit -m init
        )
        git submodule add "./s u b" "s u b" &&
        git commit -m "added submodule"
    ) &&
    : &&
    nogfsoctl init repo \
        --author="A U Thor <author@example.com>" \
        exreg --no-vid "/'${fsoExampleNs}'/d a t/gitrepoSub" &&
    nogfsoctl get repos exreg | grep "'${fsoExampleNs}'/d a t/gitrepoSub" &&
    waitGrep 5s "Confirmed init shadow" stad.log &&
    waitGrep 5s "Confirmed init GitLab" stad.log &&
    repoId=$(getRepoId exreg "/'${fsoExampleNs}'/d a t/gitrepoSub") &&
    : &&
    nogfsoctl stat --jwt-auth=no --author="A U Thor <author@example.com>" "${repoId}" &&
    waitGrep 10s "Pushed git.*/${repoId}" stad.log &&
    : &&
    logRotate stad.log &&
    nogfsoctl sha --jwt-auth=no --author="A U Thor <author@example.com>" "${repoId}" &&
    waitGrep 10s "Pushed git.*/${repoId}" stad.log &&
    : &&
    true
'

test_expect_success 'shadow git contains submodule stat blob' '
    shadowGit="$(find shadow -type d | grep "gitrepoSub/[0-9a-f-]*.fso/.git$")" &&
    git -C "${shadowGit}" show "master-stat:s u b" \
    | egrep "^submodule: \"[0-9a-f]{40}\"$" &&
    true
'

test_expect_success 'shutdown fsods' 'shutdownFsods'
test_done
