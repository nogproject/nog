# Package `nog-errata`

Due to a bug in the client-side SHA1 computation in browsers, correct blob data
was stored under an incorrect blob id in a few cases during early development.
The blobs and objects became part of the commit history.  We wanted to keep the
history but somehow mark the incorrect objects.

Since entries are immutable, the inconsistent ids cannot be modified but must
remain part of the immutable history.  To handle such situations, content
entries can have an optional field `errata`.  Example:

```json
{
    "errata": [{ "code": "ERA201609a" }]
}
```

The meaning of the errata code is deployment-specific.  The recommended format
is `ERA<year-month-char>`, for example `ERA201609a'.  Admins can use this key
to document deployment-specific information about the issue.

This package provides utilities to display errata in the UI, based on
a description in the public settings.  Example:

```json
{
    "public": {
        "errata": [
            {
                "code": "ERA201609a",
                "description": "An incorrect data checksum has been stored for this file during upload.  You can download a copy of the correct file from repo '<a href=\"/nog/era201609a/files\">nog/era201609a</a>'.  Then upload the correct file here and remove this file in order to permanently fix the issue and get rid of this message."
            }
        ]
    }
}
```
