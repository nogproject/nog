/* DO NOT EDIT.  Code generated from `propagate-acl.sh`.  To update:

   (
       sed -e '/^const propagateAclSh/,$ d' backend/internal/nogfsostaudod/propagate-acl.sh.go
       echo 'const propagateAclSh = `'
       sed -e '/^ *#/d' backend/internal/nogfsostaudod/propagate-acl.sh
       echo '`'
   ) > backend/internal/nogfsostaudod/propagate-acl.sh.go.tmp
   mv backend/internal/nogfsostaudod/propagate-acl.sh.go{.tmp,}

*/

package nogfsostaudod

const propagateAclSh = `
set -o errexit -o nounset -o pipefail -o noglob

main() {
    toplevel="$1"
    dst="$2"

    origDirAcl="$(mktemp -t 'orig-dir.acl.XXXXXXXXX')"
    dirAcl="$(mktemp -t 'dir.acl.XXXXXXXXX')"
    fileAcl="$(mktemp -t 'file.acl.XXXXXXXXX')"
    trap 'rm "${origDirAcl}" "${dirAcl}" "${fileAcl}"' EXIT

    getfacl --absolute-names --omit-header "${toplevel}" >"${origDirAcl}"

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

    (
        filteredDefaultAcl | sed -e 's/^default://'
        filteredDefaultAcl
    ) >"${dirAcl}"

    (
        grep -v '^default:' "${dirAcl}" \
        | sed --regexp-extended -e '/^(user|mask|other)/ s/x$/-/'
    ) >"${fileAcl}"

    topGroup="$(stat -c %G "${toplevel}")"
    find "${dst}" -not -group "${topGroup}" -print0 \
    | xargs -0 --no-run-if-empty \
    chgrp --no-dereference "${topGroup}" --

    find "${dst}" -type d -not -perm -g+s -print0 \
    | xargs -0 --no-run-if-empty chmod g+s --

    find "${dst}" -type d -print0 \
    | xargs -0 --no-run-if-empty setfacl --modify-file="${dirAcl}" --

    find "${dst}" -type f -print0 \
    | xargs -0 --no-run-if-empty setfacl --modify-file="${fileAcl}" --
}

main "$@"
`
