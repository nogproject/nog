package main

import "strings"

var readmeMd = qqBackticks(strings.TrimSpace(`
README tartt
============

This archive has been created with tartt.  Access the tar stream as follows.

If it has been stored with ''--gpg'' and a plaintext secret:

    cat data.tar.gpg \
    | gpg2 --batch --decrypt \
        --passphrase-file secret \
    | tar -tvf-

If it has been stored with ''--gpg'' and an encrypted secret:

    cat data.tar.gpg \
    | gpg2 --batch --decrypt \
        --passphrase-file <(gpg2 --decrypt -o- secret.asc) \
    | tar -tvf-

If it has been stored with ''--split-zstd-gpg-split'' and a plaintext secret:

    cat data.tar.zst.gpg.tar.* | tar -xOf- \
    | gpg2 --batch --decrypt --allow-multiple-messages \
        --passphrase-file secret \
    | unzstd \
    | tar -tvf-

If it has been stored with ''--split-zstd-gpg-split'' and an encrypted secret:

    cat data.tar.zst.gpg.tar.* | tar -xOf- \
    | gpg2 --batch --decrypt --allow-multiple-messages \
        --passphrase-file <(gpg2 --decrypt -o- secret.asc) \
    | unzstd \
    | tar -tvf-

If it has been stored with ''--split-zstd-split'':

    cat data.tar.zst.tar.* | tar -xOf- | unzstd | tar -tvf-

If it has been stored with ''--split-gzip-split'':

    cat data.tar.gz.tar.* | tar -xOf- | gunzip | tar -tvf-

To authenticate data that has been signed with ''tartt sign'':

    gpg2 --verify manifest.shasums{.asc,}
    grep ^sha256: manifest.shasums | cut -d : -f 2 | sha256sum -c
    grep ^sha512: manifest.shasums | cut -d : -f 2 | sha512sum -c

`)) + "\n"
