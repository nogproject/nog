# Contributing to Nog
By Steffen Prohaska
<!--@@VERSIONINC@@-->

The project ended in 2019.  There are no plans to continue development.

## Contacts

Maintainer:

* Steffen Prohaska <prohaska@zib.de>

Main contributors:

* Steffen Prohaska <prohaska@zib.de>
* Uli Homberg <homberg@zib.de>

Former main contributors:

* Vincent Dercksen <dercksen@zib.de>
* Marc Osterland <osterland@zib.de>

## Reporting an issue with Nog

We have not yet established an automated issue tracking process.  For now,
please directly contact one of the Main Contributors to discuss your issue.

## Developing Nog

Npm scripts are used for JavaScript.  Make is used for Go.  The two approaches
should be incrementally unified when there are opportunities.

[HACKING](./HACKING.md) contains general and JavaScript-specific instructions
how to setup a develop environment for Nog.  [HACKING-go](./HACKING-go.md)
contains Go-specific instructions.

## Submitting pull requests

Before you submit code, please confirm that you have the right to contribute it
as open source.  We use a sign-off procedure similar to the Linux kernel.  The
rules are pretty simple: if you can certify the below (from
<http://developercertificate.org>):

    Developer Certificate of Origin
    Version 1.1

    Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
    660 York Street, Suite 102,
    San Francisco, CA 94110 USA

    Everyone is permitted to copy and distribute verbatim copies of this
    license document, but changing it is not allowed.


    Developer's Certificate of Origin 1.1

    By making a contribution to this project, I certify that:

    (a) The contribution was created in whole or in part by me and I
        have the right to submit it under the open source license
        indicated in the file; or

    (b) The contribution is based upon previous work that, to the best
        of my knowledge, is covered under an appropriate open source
        license and I have the right under that license to submit that
        work with modifications, whether created in whole or in part
        by me, under the same open source license (unless I am
        permitted to submit under a different license), as indicated
        in the file; or

    (c) The contribution was provided directly to me by some other
        person who certified (a), (b) or (c) and I have not modified
        it.

    (d) I understand and agree that this project and the contribution
        are public and that a record of the contribution (including all
        personal information I submit with it, including my sign-off) is
        maintained indefinitely and may be redistributed consistent with
        this project or the open source license(s) involved.

then you just add a line to the commit message saying:

    Signed-off-by: Random J Developer <random@developer.example.org>

using your real name.  Ideally, you also sign your commit with your GPG key.
Technically, use at least `git commit -s`, ideally use `git commit -s -S`.

You sometimes need to slightly modify commits created by others.  Rule (b)
allows you to do that.  But it is considered impolite to simply change the
original code without indication.  We suggest that you add an indication of
your last-minute changes in the footer, prefixed with your initials, like so:

    [lkm: added detail foobar.]

    Signed-off-by: Random J Developer <random@developer.example.org>
    Signed-off-by: Lucky K Maintainer <lucky@maintainer.example.org>

We manage `master` as a sequence of merges of feature branches.

To propose commits, create a Git branch that is based on `master`.  Push your
branch to `p/<topic>`; `p` like pull request or proposal.  Ask someone to
review your branch.  Then ask the Maintainer to merge.

We keep pull requests for a while on `next` before they get merged to `master`.
The latest candidate merge sequence for master is along the first parent
`next^`.  The second parent `next^2` points to the history of candidates.

Consider the same style of chained merges to maintain pull request revisions,
using the `git tie` helper.  See [Git tooling](#git-tooling) below.
Force-pushing pull request revisions is also acceptable.

Merged p/ branches are collected on a special branch `attic` in order to keep
a reference to the p/ branch history. The p/ branch is then deleted.

If we decide against merging a p/ branch but do not simply want to delete it,
because we think that parts of it might become relevant in the future, we
revise the branch to only add a note to `LOG.md` that refers to previous p/
branch commits.  We then merge the note instead of the implementation.  The p/
branch gets garbage collected to `attic` as part of the usual workflow, which
ensures that references from the LOG to the implementation commits remain
valid.

## Git tooling

`tools/bin/git-tie` can be used to maintain a history of rebased versions of
a pull request or any branch in general.  See `tools/bin/git-tie -h` for
details.

See <https://mikegerwitz.com/papers/git-horror-story> for a comprehensive
discussion of GPG signatures in Git.

`git scommit` is an alias for sign-off and gpg-sign:

```
scommit = commit -s -S
```

`git smerge` is useful for managing next.  It creates gpg-signed
non-fast-forward merges with a minimal commit message.  It checks that the
first parent of a tied branch (see above) is merged and not the tie commit
itself:

<!-- XXX keep tabs! -->
```
	smerge = "!f() { : merge && \
			if git show -s --pretty='%s' | grep -i '^tie' ; then \
				echo >&2 \"Error: HEAD looks like a tie merge; maybe 'git reset HEAD^'?\" ; \
				false ; \
			fi && \
			name=\"$1\" && \
			if git show -s --pretty='%s' \"${name}\" | grep -i '^tie' ; then \
				echo >&2 \"Error: Looks like a tie merge; did you mean '${name}^'?\" ; \
				false ; \
			fi && \
			sname=\"${name#origin/}\" && \
			sname=\"${sname%^}\" && \
			git merge -S --no-ff \"${name}\" -m \"Merge '${sname}'\" --log --edit ; \
		} ; f"
```

The maintainer uses `tools/bin/git-p-gc` to garbage collect p/ branches that
have been merged to master.  `git-p-gc` automatically maintains `attic`.
