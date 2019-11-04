#!/bin/bash
# vim: sw=4
set -o errexit -o nounset -o pipefail -o noglob

# Based on bcpfs-propagate-toplevel-acls propagateToplevelAcl().  Differences:
#
#  - no dry run;
#  - no ACL diff;
#  - named user entries are ignored.
#
main() {
    toplevel="$1"
    dst="$2"

    origDirAcl="$(mktemp -t 'orig-dir.acl.XXXXXXXXX')"
    dirAcl="$(mktemp -t 'dir.acl.XXXXXXXXX')"
    fileAcl="$(mktemp -t 'file.acl.XXXXXXXXX')"
    trap 'rm "${origDirAcl}" "${dirAcl}" "${fileAcl}"' EXIT

    getfacl --absolute-names --omit-header "${toplevel}" >"${origDirAcl}"

    # Propagate filtered default ACL.  Modify only entries for which
    # bcpfs-perms is responsible: the unnamed entries, the owning group, and
    # X-ops groups.  Named user entries are ignored.
    owningGroup=$(stat -c %G "${toplevel}")
    filteredDefaultAcl() {
        cat "${origDirAcl}" \
        | egrep \
            -e '^default:user::' \
            -e '^default:group::' \
            -e "^default:group:${owningGroup}:" \
            -e '^default:group:[^:]+-ops:' \
            -e '^default:mask::' \
            -e '^default:other::'
    }

    # The new directory ACL has two parts:
    #
    #  - the parent default ACL becomes the normal ACL;
    #  - the parent default ACL is propagated.
    #
    (
        filteredDefaultAcl | sed -e 's/^default://'
        filteredDefaultAcl
    ) >"${dirAcl}"

    # Remove default entries and x-bit for the file ACL.
    (
        grep -v '^default:' "${dirAcl}" \
        | sed --regexp-extended -e '/^(user|mask|other)/ s/x$/-/'
    ) >"${fileAcl}"

    # Owning group.  Run `chgrp` only if necessary, in order to avoid
    # unnecessary ctime changes.  `chgrp` always updates the ctime even if the
    # group is unmodified.
    topGroup="$(stat -c %G "${toplevel}")"
    find "${dst}" -not -group "${topGroup}" -print0 \
    | xargs -0 --no-run-if-empty \
    chgrp --no-dereference "${topGroup}" --

    # SGID.  Run `chmod` only if necessary, in order to avoid unnecessary ctime
    # changes.  `chmod` always updates the ctime even if the permissions are
    # unmodified.
    find "${dst}" -type d -not -perm -g+s -print0 \
    | xargs -0 --no-run-if-empty chmod g+s --

    # Modify dir ACLs `dirAcl`.
    find "${dst}" -type d -print0 \
    | xargs -0 --no-run-if-empty setfacl --modify-file="${dirAcl}" --

    # Modify file ACLs `fileAcl`.
    find "${dst}" -type f -print0 \
    | xargs -0 --no-run-if-empty setfacl --modify-file="${fileAcl}" --
}

main "$@"
