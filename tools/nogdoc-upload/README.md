# nogdoc-upload

`nogdoc-upload` uploads a specified set of files to nog and puts them in the
`nog/doc` repository, replacing all existing content. This repository is
created when it does not exists yet. 

The files to be uploaded are specified in the list `DOCS`. Each entry consists
of a tuple with two entries: (1) the file in the file system, and (2) the
target location in the `doc` repository.

The script must be executed from the `nog-repo` directory, i.e.
`./tools/nogdoc-upload/nocdoc-upload`. The nog environment variables must be
set for user 'nog'.

