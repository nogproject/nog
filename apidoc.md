# Nog REST v1 API Documentation
By Steffen Prohaska
<!--@@VERSIONINC@@-->

<!--
    DO NOT EDIT.  The documentation has been automatically created from the
    package READMEs by gen-devdoc.
-->

<!-- toc -->

## Introduction

### Endpoint

The API is accessible at the same URL as the app, using a prefix.  The paths
for the API methods are described relative to the prefix.  Clients are
encouraged to request a specific API version.

The current version is at:

    https://<host>/api/v1

### Data Model

Data is organized in repos, like in git.  A repo contains mutable state,
primarily refs.  refs are like git refs.  By convention, we currently use only
`branches/master` (which corresponds to `heads/master` in git).  We will
probably define a meaning for other prefixes later (like tags).

As in git, a ref points to a commit.  Like a git commit, a nog commit contains
information about authors, dates, a message, and so on.  A commit points to
parent commits and to a tree.  Unlike a git commit, a nog commit can contain
a dictionary of metadata, which may be useful when implementing specific
workflows.

A tree has a name, a dictionary of metadata, and a list of child `entries` of
format `{type: object|tree, sha1: <id>}`.  Tree is a recursive data structure.
The leaf nodes are objects.  Objects also have a name and a metadata
dictionary; and they can point to a blob.  A blob represents a binary object
that is stored in object storage (currently S3).

Trees can be used similar to a file system or a git tree.  A main difference is
that the nog tree entries are an ordered list and may contain entries with
duplicate names.  By convention, we usually avoid duplicate names and use
a tree like a hierarchical file system.

We use conventions to give some trees and objects a certain meaning.  Objects
with name `*.md` are assumed to contain markdown in `text` with `blob=null`.
Objects that are named like an image file, like `*.png`, are expected to have
a blob that contains the binary image data.  An example for a convention on
trees is `meta.workspace`.  If it is present, the tree is expected to represent
a workspace with certain entries, like `datalist`, `programs`, `jobs`, and
`results`.

Immutable content has an id that is computed as the sha1 over a canonical JSON
format (see below for technical details).  The documents stored in the database
may contain additional non-essential fields that are not part of the canonical
format.  The most obvious example is `_id`, which is the computed sha1.

Updates to a repo work similar to git on a low level: Get the ref, then the
commit for `branches/master`.  Then get the tree and modify it; or construct
a new tree from scratch; only the result matters.  Construct a commit that
points to the tree and to the previous commit.  Post everything in dependency
order, and finally update the ref, passing in the previous state as a nonce in
order to protect against concurrent writes.  It should be clear how to do
this with the API routes below.  Language bindings may offer convenience
functions that operate on a higher level and use caching.  Since all content
is immutable (except for repos), caching is easy.

### Content Ids

All immutable content entries (such as objects, trees and commits) have ids
that are computed as sha1s over a canonical EJSON format.  The input `content`
is a minimal format (without `href`) that includes all optional fields.  See
examples below at 'create a commit', 'create an object', and 'create a tree'.
The canonical EJSON format is JSON with UTF-8-encoded strings, sorted keys, and
separators without whitespace.

There may be several different canonical formats for entry types.  The format
version that must be used to reproduce the sha1 id is indicated by an integer
`_idversion`.  Clients should always be updated as soon as possible to handle
new versions correctly and keep code to handle older versions for
compatibility.  Clients should check the `_idversion` and handle an unknown
version as an error.

The details for each version are documented below at the respective 'create
a ...' sections.  Briefly:

 - Commit format 0 supported only UTC Z date times.
 - Commit format 1 added timezone support.
 - Object format 0 by convention used `meta.content` for fulltext.
 - Object format 1 added a toplevel field `text` to store fulltext.
 - Tree format 0 is the only tree format.

`_idversion` is not part of the canonical content and must be removed before
computing the sha1.  `errata` (see below) must also be removed.

Computing an id in CoffeeScript:

```{.coffee}
sha1Hex = (d) -> CryptoJS.SHA1(d).toString()
contentId = (content) -> sha1Hex(EJSON.stringify(content, {canonical: true}))
```

Computing an id in Python:

```{.python}
def stringify_canonical(content):
    return json.dumps(
        content, sort_keys=True, ensure_ascii=False, separators=(',', ':'),
    ).encode('utf-8')

def contentId(content):
    h = hashlib.sha1()
    h.update(stringify_canonical(content))
    return h.hexdigest()
```

### Errata

Due to a bug in the client-side SHA1 computation in browsers, correct blob data
was stored under an incorrect blob id in a few cases during early development.
The blobs and objects became part of the commit history.  We wanted to keep the
history but somehow mark the incorrect objects.

Since entries are immutable, the inconsistent ids cannot be modified but must
remain part of the immutable history.  To handle such situations, content
entries can have an optional field `errata` with a list of errata codes.
`errata` must be removed when verifying the entry's id.  The meaning of the
errata codes is deployment-specific.

### Authentication

The API uses a digital signature for authentication that is appended to the URL
as a query string.

The following CoffeeScript code implements the signature process:

```{.coffee}
## Encode without ':' and strip milliseconds, since they are irrelevant.
toISOStringUrlsafe = (date) -> date.toISOString().replace(/:|\.[^Z]*/g, '')

NogAuth.signRequest = (key, req) ->
  authalgorithm = 'nog-v1'
  authkeyid = key.keyid
  now = new Date()
  authdate = toISOStringUrlsafe(now)
  authexpires = config.defaultExpires
  authnonce = crypto.randomBytes(10).toString('hex')
  if urlparse(req.url).query?
    req.url += '&'
  else
    req.url += '?'
  req.url += "authalgorithm=#{authalgorithm}"
  req.url += '&' + "authkeyid=#{authkeyid}"
  req.url += '&' + "authdate=#{authdate}"
  req.url += '&' + "authexpires=#{authexpires}"
  req.url += '&' + "authnonce=#{authnonce}"

  stringToSign = req.method + "\n" + req.url + "\n"
  hmac = crypto.createHmac 'sha256', key.secretkey
  hmac.update stringToSign
  authsignature = hmac.digest 'hex'

  req.url += '&' + "authsignature=#{authsignature}"
```

The method and the whole URL path are signed.  The `authsignature` must be
appended as the last query parameter.

`authexpires` is specified in seconds.

The `authnonce` is optional.  If it is present, the request will be accepted
only once.  The `authnonce` needs to be unique only per `authdate`, so a small
nonce is usually sufficient.

`sign-req`, available from the
[nog-starter-pack](/nog/packages/files/programs/nog-starter-pack/index!0/content.tar.xz),
can be used to sign requests for curl:

Example:

```{.bash}
export NOG_KEYID=<copied>
export NOG_SECRETKEY=<copied>

curl $(
  ./tools/bin/sign-req GET \
  http://localhost:3000/api/blobs/31968d2e8b58e29e63851cb4b340216026f11f69
) | python -m json.tool
```

The following code implements the signature process in Python:

```{.python}
def sign_req(method, url):
    authkeyid = os.environ['NOG_KEYID']
    secretkey = os.environ['NOG_SECRETKEY'].encode()
    authalgorithm = 'nog-v1'
    authdate = datetime.utcnow().strftime('%Y-%m-%dT%H%M%SZ')
    authexpires = '600'
    authnonce = binascii.hexlify(os.urandom(5))

    parsed = urlparse(url)
    if parsed.query == '':
        path = parsed.path
        suffix = '?'
    else:
        path = parsed.path + '?' + parsed.query
        suffix = '&'
    suffix = suffix + 'authalgorithm=' + authalgorithm
    suffix = suffix + '&authkeyid=' + authkeyid
    suffix = suffix + '&authdate=' + authdate
    suffix = suffix + '&authexpires=' + authexpires
    suffix = suffix + '&authnonce=' + authnonce

    stringToSign = (method + '\n' + path + suffix + '\n').encode()
    authsignature = hexlify(hmac.new(
            secretkey, stringToSign, digestmod=hashlib.sha256
        ).digest()).decode()
    suffix = suffix + '&authsignature=' + authsignature
    return url + suffix
```

## API



<!--
DO NOT EDIT.
This file has been automatically generated by gen-apidoc-nog-content.
-->

### Create a Repo

```
POST /repos
```

**Request body**

 - `repoFullName` (`String` of format `<owner>/<name>`): The name of
   the repository

Example:

```json
{
  "repoFullName": "fred/hello-world"
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world",
      "id": "8rNFhDWE6x42io2Hq"
    },
    "fullName": "fred/hello-world",
    "name": "hello-world",
    "owner": "fred",
    "ownerId": "g8dB4y3DYSPQfeXkL",
    "refs": {
      "branches/master": "0000000000000000000000000000000000000000"
    }
  },
  "statusCode": 201
}
```




### Get a Reference

```
GET /repos/:repoOwner/:repoName/db/refs/:refName
```

**Response**

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/refs/branches/master",
      "refName": "branches/master"
    },
    "entry": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "type": "commit"
    }
  },
  "statusCode": 200
}
```




### Get all References

```
GET /repos/:repoOwner/:repoName/db/refs
```

**Response**

```
Status: 200
```

```json
{
  "data": {
    "count": 2,
    "items": [
      {
        "_id": {
          "href": "https://nog.zib.de/api/repos/fred/hello-world/db/refs/branches/master",
          "refName": "branches/master"
        },
        "entry": {
          "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc",
          "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
          "type": "commit"
        }
      },
      {
        "_id": {
          "href": "https://nog.zib.de/api/repos/fred/hello-world/db/refs/branches/foo/bar",
          "refName": "branches/foo/bar"
        },
        "entry": {
          "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc",
          "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
          "type": "commit"
        }
      }
    ]
  },
  "statusCode": 200
}
```




### Update a Reference

```
PATCH /repos/:repoOwner/:repoName/db/refs/:refName
```

**Request body**

 - `new` (`String`): The new value of the reference (a hex sha1).
 - `old` (`String`): The old value of the reference (a hex sha1).

The old value must be specified as a safety measure against accidentally
overwriting a reference that has been modified by someone else.  `null` or
`0000000000000000000000000000000000000000` can be used to indicate that the
reference is unset.

Example:

```json
{
  "new": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
  "old": "0000000000000000000000000000000000000000"
}
```

**Response**

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/refs/branches/master",
      "refName": "branches/master"
    },
    "entry": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "type": "commit"
    }
  },
  "statusCode": 200
}
```




### Delete a Reference

```
DELETE /repos/:repoOwner/:repoName/db/refs/:refName
```

**Request body**

 - `old` (`String`): The old value of the reference (a hex sha1).

The old value must be specified as a safety measure against accidentally
deleting a reference that has been modified by someone else.

Example:

```json
{
  "old": "7215f2bb2b2128da2abb00b90e2be2f0274016cc"
}
```

**Response**

```
Status: 204
```




### Get a Commit

```
GET /repos/:repoOwner/:repoName/db/commits/:sha1?format=:format
```

**Request query params**

 - `format=:format` (`minimal` or `hrefs` with optional suffix `.v0` of `.v1`;
   default: `hrefs`): Specifies whether the result contains a minimal
   representation or embedded hrefs.  The suffix specifies which representation
   version to return.  The default without suffix is to return the
   representation that matches `_idversion`.

**Response**

With hrefs:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc?format=hrefs
```

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc"
    },
    "_idversion": 1,
    "authorDate": "2016-02-18T06:14:20+00:00",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2016-02-18T06:14:20+00:00",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {
      "importGitCommit": "1919191919191919191919191919191919191919"
    },
    "parents": [
      {
        "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/6812c564e1b0b4c4abd6d1fa75f467f0e57079d4",
        "sha1": "6812c564e1b0b4c4abd6d1fa75f467f0e57079d4"
      }
    ],
    "subject": "Initial commit",
    "tree": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94",
      "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
    }
  },
  "statusCode": 200
}
```



With hrefs, representation v0 with UTC Z datetimes:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/commits/86e03b3720b912ff3ae6de494464f8a764597778?format=hrefs.v0
```

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/86e03b3720b912ff3ae6de494464f8a764597778",
      "sha1": "86e03b3720b912ff3ae6de494464f8a764597778"
    },
    "_idversion": 0,
    "authorDate": "2015-01-01T00:00:00Z",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2015-01-01T00:00:00Z",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {},
    "parents": [],
    "subject": "Initial commit",
    "tree": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/5af3a99f790fc7cfee9622b35564585c8d4df64a",
      "sha1": "5af3a99f790fc7cfee9622b35564585c8d4df64a"
    }
  },
  "statusCode": 200
}
```



With hrefs, representation v1 with UTC timezone support:

Note that `_idversion` and format version may differ.  To compute the correct
id, the client code must convert the dates to the correct `_idversion`, which
is UTC Z for `_idversion: 0`.

```
GET http://localhost:3000/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc?format=hrefs.v1
```

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc"
    },
    "_idversion": 1,
    "authorDate": "2016-02-18T06:14:20+00:00",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2016-02-18T06:14:20+00:00",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {
      "importGitCommit": "1919191919191919191919191919191919191919"
    },
    "parents": [
      {
        "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/6812c564e1b0b4c4abd6d1fa75f467f0e57079d4",
        "sha1": "6812c564e1b0b4c4abd6d1fa75f467f0e57079d4"
      }
    ],
    "subject": "Initial commit",
    "tree": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94",
      "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
    }
  },
  "statusCode": 200
}
```



Minimal:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc?format=minimal
```

```
Status: 200
```

```json
{
  "data": {
    "_id": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
    "_idversion": 1,
    "authorDate": "2016-02-18T06:14:20+00:00",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2016-02-18T06:14:20+00:00",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {
      "importGitCommit": "1919191919191919191919191919191919191919"
    },
    "parents": [
      "6812c564e1b0b4c4abd6d1fa75f467f0e57079d4"
    ],
    "subject": "Initial commit",
    "tree": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
  },
  "statusCode": 200
}
```



Minimal, representation v0 with UTC Z datetimes:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/commits/86e03b3720b912ff3ae6de494464f8a764597778?format=minimal.v0
```

```
Status: 200
```

```json
{
  "data": {
    "_id": "86e03b3720b912ff3ae6de494464f8a764597778",
    "_idversion": 0,
    "authorDate": "2015-01-01T00:00:00Z",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2015-01-01T00:00:00Z",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {},
    "parents": [],
    "subject": "Initial commit",
    "tree": "5af3a99f790fc7cfee9622b35564585c8d4df64a"
  },
  "statusCode": 200
}
```



Minimal, representation v1 with UTC timezone support:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc?format=minimal.v1
```

```
Status: 200
```

```json
{
  "data": {
    "_id": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
    "_idversion": 1,
    "authorDate": "2016-02-18T06:14:20+00:00",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2016-02-18T06:14:20+00:00",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {
      "importGitCommit": "1919191919191919191919191919191919191919"
    },
    "parents": [
      "6812c564e1b0b4c4abd6d1fa75f467f0e57079d4"
    ],
    "subject": "Initial commit",
    "tree": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
  },
  "statusCode": 200
}
```




### Create a Commit

```
POST /repos/:repoOwner/:repoName/db/commits?format=:format
```

**Request query params**

 - `format=:format` (`minimal` or `hrefs` with optional suffix `.v0` or `.v1`;
   default: `hrefs`): Specifies whether the result is a minimal representation
   or contains embedded hrefs (see example at get).

**Request body**

The body contains a JSON representation of the commit with the following keys:

 - `subject` (`String`): The subject line of the commit.
 - `message` (`String`): The body of the commit message.
 - `tree` (`String`): The id of the tree as a hex sha1.
 - `parents` (`[String]`): The ids of the parent commits as hex sha1s.  The
   array may be empty.
 - `authors` (`[String]`, optional): An array of authors, by convention `John
   Q. Public <john@example.com>`.
 - `authorDate` (`String`, optional): An ISO string without fractional seconds
   (see below for `_idversion`).
 - `committer` (`String`, optional)
 - `commitDate` (`String`, optional): An ISO string without fractional seconds
   (see below for `_idversion`).
 - `meta` (`Object`, optional): Meta data that is stored with the commit.
 - `_idversion` (`0` or `1`, default `1`): Specify format to use for computing
   the sha1 id.

The date format differs between representation versions:

 - `_idversion 0`: Dates are UTC with Z timezone indicator.
 - `_idversion 1`: Dates use a timezone offset `+HH:MM` or `-HH:MM`.

Example:

```json
{
  "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
  "meta": {
    "importGitCommit": "1919191919191919191919191919191919191919"
  },
  "parents": [
    "6812c564e1b0b4c4abd6d1fa75f467f0e57079d4"
  ],
  "subject": "Initial commit",
  "tree": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc"
    },
    "_idversion": 1,
    "authorDate": "2016-02-18T06:14:20+00:00",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2016-02-18T06:14:20+00:00",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {
      "importGitCommit": "1919191919191919191919191919191919191919"
    },
    "parents": [
      {
        "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/6812c564e1b0b4c4abd6d1fa75f467f0e57079d4",
        "sha1": "6812c564e1b0b4c4abd6d1fa75f467f0e57079d4"
      }
    ],
    "subject": "Initial commit",
    "tree": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94",
      "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
    }
  },
  "statusCode": 201
}
```



Commit `idversion 0`:

```
POST /repos/:repoOwner/:repoName/db/commits?format=:format
```

```json
{
  "_idversion": 0,
  "authorDate": "2015-01-01T00:00:00Z",
  "commitDate": "2015-01-01T00:00:00Z",
  "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
  "parents": [],
  "subject": "Initial commit",
  "tree": "5af3a99f790fc7cfee9622b35564585c8d4df64a"
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/commits/86e03b3720b912ff3ae6de494464f8a764597778",
      "sha1": "86e03b3720b912ff3ae6de494464f8a764597778"
    },
    "_idversion": 0,
    "authorDate": "2015-01-01T00:00:00Z",
    "authors": [
      "unknown <unknown>"
    ],
    "commitDate": "2015-01-01T00:00:00Z",
    "committer": "unknown <unknown>",
    "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
    "meta": {},
    "parents": [],
    "subject": "Initial commit",
    "tree": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/5af3a99f790fc7cfee9622b35564585c8d4df64a",
      "sha1": "5af3a99f790fc7cfee9622b35564585c8d4df64a"
    }
  },
  "statusCode": 201
}
```




### Get an Object

```
GET /repos/:repoOwner/:repoName/db/objects/:sha1?format=:format
```

**Request query params**

 - `format=:format` (`minimal` or `hrefs` with optional suffix `.v0` or `.v1`;
   default: `hrefs`): Specifies whether the result contains a minimal
   representation or embedded hrefs.

**Response**

With hrefs:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/objects/d46126638a13e0b86adc09d15670c8cfeb19373b?format=hrefs
```

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/d46126638a13e0b86adc09d15670c8cfeb19373b",
      "sha1": "d46126638a13e0b86adc09d15670c8cfeb19373b"
    },
    "_idversion": 1,
    "blob": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/3f786850e387550fdab836ed7e6dc881de23001b",
      "sha1": "3f786850e387550fdab836ed7e6dc881de23001b"
    },
    "meta": {
      "random": "bukxwstgav",
      "specimen": "bar",
      "study": "foo"
    },
    "name": "Fake data",
    "text": null
  },
  "statusCode": 200
}
```



Minimal:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/objects/d46126638a13e0b86adc09d15670c8cfeb19373b?format=minimal
```

```
Status: 200
```

```json
{
  "data": {
    "_id": "d46126638a13e0b86adc09d15670c8cfeb19373b",
    "_idversion": 1,
    "blob": "3f786850e387550fdab836ed7e6dc881de23001b",
    "meta": {
      "random": "bukxwstgav",
      "specimen": "bar",
      "study": "foo"
    },
    "name": "Fake data",
    "text": null
  },
  "statusCode": 200
}
```



Minimal, explicit format version 0:

Note that `_idversion` and format version may differ.  To compute the correct
sha1, the client must convert the format to the `_idversion` that the server
reported.

```
GET http://localhost:3000/api/repos/fred/hello-world/db/objects/5541d329b004502cbed1d97f037dcf20527fd29f?format=minimal.v0
```

```
Status: 200
```

```json
{
  "data": {
    "_id": "5541d329b004502cbed1d97f037dcf20527fd29f",
    "_idversion": 0,
    "blob": "0000000000000000000000000000000000000000",
    "meta": {
      "content": "Lorem ipsum...",
      "random": "syskehmxsk"
    },
    "name": "fake-index.md"
  },
  "statusCode": 200
}
```



Minimal, explicit format version 1:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/objects/5541d329b004502cbed1d97f037dcf20527fd29f?format=minimal.v1
```

```
Status: 200
```

```json
{
  "data": {
    "_id": "5541d329b004502cbed1d97f037dcf20527fd29f",
    "_idversion": 0,
    "blob": null,
    "meta": {
      "random": "syskehmxsk"
    },
    "name": "fake-index.md",
    "text": "Lorem ipsum..."
  },
  "statusCode": 200
}
```




### Create an Object

```
POST /repos/:repoOwner/:repoName/db/objects?format=:format
```

**Request query params**

 - `format=:format` (`minimal` or `hrefs`; default: `hrefs`): Specifies whether
   the result is a minimal representation or contains embedded hrefs (see
   example at get)

**Request body**

The body contains a JSON representation of the object with the following keys:

 - `blob` (`String` or `null`): The identifier of the associated blob (a hex
   sha1).
 - `text` (`String` or `null`; since format version 1): Text content of the
   object.  Text content is indexed for fulltext search, while blob content is
   opaque.  By convention either use `blob` or `text`, or none of them; but do
   not use both at the same time.
 - `name` (`String`): The name of the object.
 - `meta` (`Object`): Meta data that is stored with the object.
 - `_idversion` (`0` or `1`, default `1`): Specify the format to use for
   computing the sha1 id.

There are two different format versions:

 - `_idversion 0`: Absence of a blob is indicated by
   `0000000000000000000000000000000000000000`.  Fulltext is, by convention,
   stored in `meta.content`.
 - `_idversion 1`: Absence of a blob is indicated by `null`.  Fulltext is
   stored in `text` (may be `null`).

Example:

```json
{
  "blob": "3f786850e387550fdab836ed7e6dc881de23001b",
  "meta": {
    "random": "elkqaanymh",
    "specimen": "bar",
    "study": "foo"
  },
  "name": "Fake data"
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/15635f828b11153643f932b3e57fd9f527a4be66",
      "sha1": "15635f828b11153643f932b3e57fd9f527a4be66"
    },
    "_idversion": 1,
    "blob": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/3f786850e387550fdab836ed7e6dc881de23001b",
      "sha1": "3f786850e387550fdab836ed7e6dc881de23001b"
    },
    "meta": {
      "random": "elkqaanymh",
      "specimen": "bar",
      "study": "foo"
    },
    "name": "Fake data",
    "text": null
  },
  "statusCode": 201
}
```



Object with idversion 0 layout:

```
POST /repos/:repoOwner/:repoName/db/objects?format=:format
```

```json
{
  "_idversion": 0,
  "blob": null,
  "meta": {
    "content": "Lorem ipsum...",
    "random": "syskehmxsk"
  },
  "name": "fake-index.md"
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/5541d329b004502cbed1d97f037dcf20527fd29f",
      "sha1": "5541d329b004502cbed1d97f037dcf20527fd29f"
    },
    "_idversion": 0,
    "blob": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/0000000000000000000000000000000000000000",
      "sha1": "0000000000000000000000000000000000000000"
    },
    "meta": {
      "content": "Lorem ipsum...",
      "random": "syskehmxsk"
    },
    "name": "fake-index.md"
  },
  "statusCode": 201
}
```




### Get a Tree

```
GET /repos/:repoOwner/:repoName/db/trees/:sha1?expand=:levels&format=:format
```

**Request query params**

 - `expand=:levels` (non-negative integer, optional): Specifies how many entry
   levels will be expanded recursively.  0 indicates no expansion.
 - `format=:format` (`minimal` or `hrefs` with optional suffix `.v0`; default:
   `hrefs`): Specifies whether the result contains a minimal representation or
   embedded hrefs.

The optional `format` version suffix may only be used with `expand=0`.
Children will always be expanded in the format that matches their `_idversion`.

**Response**

Unexpanded:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94?expand=0&format=hrefs
```

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94",
      "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
    },
    "_idversion": 0,
    "entries": [
      {
        "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/d46126638a13e0b86adc09d15670c8cfeb19373b",
        "sha1": "d46126638a13e0b86adc09d15670c8cfeb19373b",
        "type": "object"
      },
      {
        "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/b4556ff729e1d49a25cf90c19b5bf8df8ce88a4f",
        "sha1": "b4556ff729e1d49a25cf90c19b5bf8df8ce88a4f",
        "type": "object"
      }
    ],
    "meta": {
      "study": "foo"
    },
    "name": "Workspace root"
  },
  "statusCode": 200
}
```



Expanded:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94?expand=1&format=hrefs
```

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94",
      "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
    },
    "_idversion": 0,
    "entries": [
      {
        "_id": {
          "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/d46126638a13e0b86adc09d15670c8cfeb19373b",
          "sha1": "d46126638a13e0b86adc09d15670c8cfeb19373b"
        },
        "_idversion": 1,
        "blob": {
          "href": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/3f786850e387550fdab836ed7e6dc881de23001b",
          "sha1": "3f786850e387550fdab836ed7e6dc881de23001b"
        },
        "meta": {
          "random": "bukxwstgav",
          "specimen": "bar",
          "study": "foo"
        },
        "name": "Fake data",
        "text": null
      },
      {
        "_id": {
          "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/b4556ff729e1d49a25cf90c19b5bf8df8ce88a4f",
          "sha1": "b4556ff729e1d49a25cf90c19b5bf8df8ce88a4f"
        },
        "_idversion": 1,
        "blob": null,
        "meta": {
          "random": "gotlxwjvxj"
        },
        "name": "index.md",
        "text": "Lorem ipsum..."
      }
    ],
    "meta": {
      "study": "foo"
    },
    "name": "Workspace root"
  },
  "statusCode": 200
}
```



Expanded, minimal:

```
GET http://localhost:3000/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94?expand=1&format=minimal
```

```
Status: 200
```

```json
{
  "data": {
    "_id": "be9cd0d3d9150ac633e317f78d01a71f40077e94",
    "_idversion": 0,
    "entries": [
      {
        "_id": "d46126638a13e0b86adc09d15670c8cfeb19373b",
        "_idversion": 1,
        "blob": "3f786850e387550fdab836ed7e6dc881de23001b",
        "meta": {
          "random": "bukxwstgav",
          "specimen": "bar",
          "study": "foo"
        },
        "name": "Fake data",
        "text": null
      },
      {
        "_id": "b4556ff729e1d49a25cf90c19b5bf8df8ce88a4f",
        "_idversion": 1,
        "blob": null,
        "meta": {
          "random": "gotlxwjvxj"
        },
        "name": "index.md",
        "text": "Lorem ipsum..."
      }
    ],
    "meta": {
      "study": "foo"
    },
    "name": "Workspace root"
  },
  "statusCode": 200
}
```




### Create a Tree

```
POST /repos/:repoOwner/:repoName/db/trees?format=:format
```

**Request query params**

 - `format=:format` (`minimal` or `hrefs`; default: `hrefs`): Specifies whether
   the result is a minimal representation or contains embedded hrefs (see
   example at get)

**Request body**

The body contains a JSON representation of the tree with the following keys:

 - `tree.name` (`String`): The name of the object.
 - `tree.meta` (`Object`): Meta data that is stored with the object.
 - `tree.entries` (`Array` of entries): An entry can either be collapsed
   `{"type": <type>, "sha1": <sha1>}`, where `<type>` can be `'object'` or
   `'tree'` and `<sha1>` must be the id of a corresponding entry; or an entry
   can contain the full content for an object or tree.

There is only a single canonical representation (`_idversion: 0`) for trees.

Trees may recursively contain trees up to the total request limit.  Consider
using a series of bulk posts (see below) if the total tree size exceeds the
limit.

Example with collapsed entry:

```json
{
  "tree": {
    "entries": [
      {
        "sha1": "15635f828b11153643f932b3e57fd9f527a4be66",
        "type": "object"
      }
    ],
    "meta": {
      "study": "foo"
    },
    "name": "Workspace root"
  }
}
```


Example with expanded entry:

```json
{
  "tree": {
    "entries": [
      {
        "blob": "3f786850e387550fdab836ed7e6dc881de23001b",
        "meta": {
          "random": "bukxwstgav",
          "specimen": "bar",
          "study": "foo"
        },
        "name": "Fake data"
      },
      {
        "_idversion": 1,
        "blob": null,
        "meta": {
          "random": "gotlxwjvxj"
        },
        "name": "index.md",
        "text": "Lorem ipsum..."
      }
    ],
    "meta": {
      "study": "foo"
    },
    "name": "Workspace root"
  }
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/trees/be9cd0d3d9150ac633e317f78d01a71f40077e94",
      "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94"
    },
    "_idversion": 0,
    "entries": [
      {
        "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/d46126638a13e0b86adc09d15670c8cfeb19373b",
        "sha1": "d46126638a13e0b86adc09d15670c8cfeb19373b",
        "type": "object"
      },
      {
        "href": "https://nog.zib.de/api/repos/fred/hello-world/db/objects/b4556ff729e1d49a25cf90c19b5bf8df8ce88a4f",
        "sha1": "b4556ff729e1d49a25cf90c19b5bf8df8ce88a4f",
        "type": "object"
      }
    ],
    "meta": {
      "study": "foo"
    },
    "name": "Workspace root"
  },
  "statusCode": 201
}
```




### Bulk Create Entries and Copy from Other Repos

```
POST /repos/:repoOwner/:repoName/db/bulk
```

**Request body**

 - `entries` (Array of expanded entries or copy instructions):  Expanded
   entries can be objects, trees, and commits.  If entries depend on each
   other, the entries must be ordered such that entries that depend on other
   entries come after their dependencies.  A special kind of entry can be used
   to copy content from other repos: `{"copy": {"type": String, "sha1":
   String, "repoFullName": String}}` copies the entry with the `sha1` from
   the repo with name `repoFullName`.  `type` can be `object`, `tree`,
   `commit`, or `blob`.

Example:

```json
{
  "entries": [
    {
      "blob": "3f786850e387550fdab836ed7e6dc881de23001b",
      "meta": {
        "random": "elkqaanymh",
        "specimen": "bar",
        "study": "foo"
      },
      "name": "Fake data"
    },
    {
      "entries": [
        {
          "sha1": "15635f828b11153643f932b3e57fd9f527a4be66",
          "type": "object"
        }
      ],
      "meta": {
        "study": "foo"
      },
      "name": "Workspace root"
    },
    {
      "message": "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed\ndo eiusmod tempor incididunt ut labore et dolore magna aliqua.\nUt enim ad minim veniam, quis nostrud exercitation ullamco\nlaboris nisi ut aliquip ex ea commodo consequat.\n",
      "meta": {
        "importGitCommit": "1919191919191919191919191919191919191919"
      },
      "parents": [
        "f14b966459667078910b9a8fcf77b5f3228f7f1e"
      ],
      "subject": "Initial commit",
      "tree": "5af3a99f790fc7cfee9622b35564585c8d4df64a"
    },
    {
      "copy": {
        "repoFullName": "fred/hello-world",
        "sha1": "15635f828b11153643f932b3e57fd9f527a4be66",
        "type": "object"
      }
    },
    {
      "copy": {
        "repoFullName": "fred/hello-world",
        "sha1": "5af3a99f790fc7cfee9622b35564585c8d4df64a",
        "type": "tree"
      }
    }
  ]
}
```

**Response**

An array of collapsed `entries` of format `{"type": String, "sha1": String}`.

```
Status: 201
```

```json
{
  "data": {
    "entries": [
      {
        "sha1": "15635f828b11153643f932b3e57fd9f527a4be66",
        "type": "object"
      },
      {
        "sha1": "5af3a99f790fc7cfee9622b35564585c8d4df64a",
        "type": "tree"
      },
      {
        "sha1": "a4e46e4265fc4dd0169cdc17001f9275aa739255",
        "type": "commit"
      },
      {
        "sha1": "15635f828b11153643f932b3e57fd9f527a4be66",
        "type": "object"
      },
      {
        "sha1": "5af3a99f790fc7cfee9622b35564585c8d4df64a",
        "type": "tree"
      }
    ]
  },
  "statusCode": 201
}
```




### Get Entry Status Information

```
POST /repos/:repoOwner/:repoName/db/stat
```

The method is `POST`, because it seems controversial whether request bodies
should be used with `GET`.

**Request body**

 - `entries` (Array of `{"type": String, "sha1": String}`): The entries for
   which status information is requested.  `type` can be `object`, `tree`,
   `commit`, or `blob`.  `sha1` is a hex sha1 id of the entry.

Example:

```json
{
  "entries": [
    {
      "sha1": "d46126638a13e0b86adc09d15670c8cfeb19373b",
      "type": "object"
    },
    {
      "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94",
      "type": "tree"
    },
    {
      "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
      "type": "commit"
    },
    {
      "sha1": "0123012301230123012301230123012301230123",
      "type": "object"
    }
  ]
}
```

**Response**

The `entries` array that was posted is echoed back with an additional field
`status` that contains `exists` or `unknown`.

```
Status: 200
```

```json
{
  "data": {
    "entries": [
      {
        "sha1": "d46126638a13e0b86adc09d15670c8cfeb19373b",
        "status": "exists",
        "type": "object"
      },
      {
        "sha1": "be9cd0d3d9150ac633e317f78d01a71f40077e94",
        "status": "exists",
        "type": "tree"
      },
      {
        "sha1": "7215f2bb2b2128da2abb00b90e2be2f0274016cc",
        "status": "exists",
        "type": "commit"
      },
      {
        "sha1": "0123012301230123012301230123012301230123",
        "status": "unknown",
        "type": "object"
      }
    ]
  },
  "statusCode": 200
}
```





<!--
DO NOT EDIT.
This file has been automatically generated by gen-apidoc-nog-blob-blobs.
-->

### Get a Blob

```
GET /repos/:repoOwner/:repoName/db/blobs/:sha1
```

**Response**

```
Status: 200
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/3f786850e387550fdab836ed7e6dc881de23001b",
      "id": "3f786850e387550fdab836ed7e6dc881de23001b"
    },
    "content": {
      "href": "https://some-s3-bucket.amazonaws.com/3f786850e387550fdab836ed7e6dc881de23001b?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKXXXXXXXXXXXXXXXXXX%2F20160218%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20160218T060613Z&X-Amz-Expires=900&X-Amz-Signature=cebc1eebb2665ed09b3460ef7c6dffaf3a13fb1f9f37fe83ac43bd519fdf9b7b&X-Amz-SignedHeaders=host&response-content-disposition=attachment%3B%20filename%3D%223f786850e387550fdab836ed7e6dc881de23001b.dat%22"
    },
    "sha1": "3f786850e387550fdab836ed7e6dc881de23001b",
    "size": 2,
    "status": "available"
  },
  "statusCode": 200
}
```




### Get the Blob Content

```
GET /repos/:repoOwner/:repoName/db/blobs/:sha1/content
```

It will respond with a redirect to S3.  The S3 URL will expire after a few
minutes.

**Response**

```
Status: 307
Location: https://some-s3-bucket.amazonaws.com/3f786850e387550fdab836ed7e6dc881de23001b?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKXXXXXXXXXXXXXXXXXX%2F20160218%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20160218T060614Z&X-Amz-Expires=900&X-Amz-Signature=2df6dec751ae156f4559178d669b3c6e663ac9fd5a3446a181f475632d3d02f9&X-Amz-SignedHeaders=host&response-content-disposition=attachment%3B%20filename%3D%223f786850e387550fdab836ed7e6dc881de23001b.dat%22
```





<!--
DO NOT EDIT.
This file has been automatically generated by gen-apidoc-nog-blob-upload.
-->

### Upload Blob Data

Uploading data requires a sequence of coordinated requests.  The upload starts
with a `POST`, which returns an upload id and descriptions how parts should be
upload to S3.  Depending on the total upload size, multiple parts need to be
uploaded to S3.  More upload parts should be requested only when needed and
used immediately, because the S3 URLs expire after a couple of minutes.  After
uploading the binary data to S3, the upload is completed by posting the `ETag`
headers that S3 returned to the completion href that has been returned by the
initial `POST`.

#### Start Upload

```
POST /repos/:repoOwner/:repoName/db/blobs/:sha1/uploads?limit=:limit
```

The sha1 of the file is specified in the path.

`:limit` is used to restrict the number of upload parts that are initially
returned.  Use a limit of 1 if your upload code works sequentially.  You may
use a larger number if your upload code handles multiple concurrent uploads to
S3.

**Request body**

 - `size` (`Number`): The file size.
 - `name` (`String`): The local file name.

Example:

```
POST https://nog.zib.de/api/repos/fred/hello-world/db/blobs/4c187d0d3e1df64c6e6365be78c13c276ff4cba4/uploads?limit=1
```

```json
{
  "name": "testdata.dat",
  "size": 6000000
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "parts": {
      "count": 2,
      "items": [
        {
          "end": 5242880,
          "href": "https://some-s3-bucket.amazonaws.com/4c187d0d3e1df64c6e6365be78c13c276ff4cba4?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKXXXXXXXXXXXXXXXXXX%2F20160218%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20160218T060332Z&X-Amz-Expires=900&X-Amz-Signature=f470a86511de484d603f743d911d2ecf250248d47ac5214ae5cc670fa717aae2&X-Amz-SignedHeaders=host&partNumber=1&uploadId=JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1",
          "partNumber": 1,
          "start": 0
        }
      ],
      "limit": 1,
      "next": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/4c187d0d3e1df64c6e6365be78c13c276ff4cba4/uploads/JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1/parts?offset=1&limit=1",
      "offset": 0
    },
    "upload": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/4c187d0d3e1df64c6e6365be78c13c276ff4cba4/uploads/JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1",
      "id": "JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1"
    }
  },
  "statusCode": 201
}
```




#### Upload Individual Parts to S3

```
PUT <data.parts.items[n].href>
```

Binary data for each part must be send to S3 using `PUT`.  The data slice for
each part is specified as `[start; end[` in the part descriptions.  The start
is inclusive; the end is exclusive.

Pay attention to the indexing: S3 part numbers start with index 1.  The
pagination starts with index 0.

Example:

```
PUT https://some-s3-bucket.amazonaws.com/4c187d0d3e1df64c6e6365be78c13c276ff4cba4?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKXXXXXXXXXXXXXXXXXX%2F20160218%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20160218T060355Z&X-Amz-Expires=900&X-Amz-Signature=fb376d6652691e44a0c28627d7adb1d0daaca97e5ca210f0a833498bab9bf721&X-Amz-SignedHeaders=host&partNumber=2&uploadId=JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1

Body with binary data for [start; end[
```

**Response**

```
Status: 200
ETag: "a6c5e0b78ec35e11070a7350daa82211"
```




#### Get Additional Part Descriptions

```
GET <parts.next>
```

More parts can be retrieved from the `next` href of the previous parts.  The
URL automatically uses the same limit as the initial request.

An alternative is to explicitly construct the URL to get more parts:

```
GET /repos/:repoOwner/:repoName/db/blobs/:sha1/uploads/:uploadId?offset=:offset&limit=:limit
```

Example:

```
GET https://nog.zib.de/api/repos/fred/hello-world/db/blobs/4c187d0d3e1df64c6e6365be78c13c276ff4cba4/uploads/JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1/parts?offset=1&limit=1
```

**Response**

```
Status: 200
```

```json
{
  "data": {
    "count": 2,
    "items": [
      {
        "end": 6000000,
        "href": "https://some-s3-bucket.amazonaws.com/4c187d0d3e1df64c6e6365be78c13c276ff4cba4?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKXXXXXXXXXXXXXXXXXX%2F20160218%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20160218T060355Z&X-Amz-Expires=900&X-Amz-Signature=fb376d6652691e44a0c28627d7adb1d0daaca97e5ca210f0a833498bab9bf721&X-Amz-SignedHeaders=host&partNumber=2&uploadId=JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1",
        "partNumber": 2,
        "start": 5242880
      }
    ],
    "limit": 1,
    "next": null,
    "offset": 1
  },
  "statusCode": 200
}
```




#### Complete the Upload

The upload is completed by a `POST` to the href that has been returned as
`upload.href` when starting the upload.

```
POST <upload.href>
```

The format is:

```
POST /repos/:repoOwner/:repoName/db/blobs/:sha1/uploads/:uploadId
```

Example:

```
POST https://nog.zib.de/api/repos/fred/hello-world/db/blobs/4c187d0d3e1df64c6e6365be78c13c276ff4cba4/uploads/JbzKJWaG0.Q8kcQFLrb0wwfG_VNTq_ZT_W9JqtPtEyrob6RKKBiAXGRH717QwqzPvJopYTgCDD_2rfhwPjDnpiYkVE17fvSS6hIDFQkOyxwb.y5UT8hGJM4dGFbj8SO1
```

**Request body**

The body contains a field `s3Parts` with an array with the part numbers and
ETag headers returned by S3.

Example:

```json
{
  "s3Parts": [
    {
      "ETag": "\"8220aff7f4c6452a8e7f8cd1be261365\"",
      "PartNumber": 1
    },
    {
      "ETag": "\"a6c5e0b78ec35e11070a7350daa82211\"",
      "PartNumber": 2
    }
  ]
}
```

**Response**

```
Status: 201
```

```json
{
  "data": {
    "_id": {
      "href": "https://nog.zib.de/api/repos/fred/hello-world/db/blobs/4c187d0d3e1df64c6e6365be78c13c276ff4cba4",
      "id": "4c187d0d3e1df64c6e6365be78c13c276ff4cba4"
    },
    "content": {
      "href": "https://some-s3-bucket.amazonaws.com/4c187d0d3e1df64c6e6365be78c13c276ff4cba4?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKXXXXXXXXXXXXXXXXXX%2F20160218%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20160218T060401Z&X-Amz-Expires=900&X-Amz-Signature=8ac8c68a1f72db5a4e4e03c443f095b2dc6236d3f4a0a3803d888c1e6c8e7414&X-Amz-SignedHeaders=host&response-content-disposition=attachment%3B%20filename%3D%224c187d0d3e1df64c6e6365be78c13c276ff4cba4.dat%22"
    },
    "sha1": "4c187d0d3e1df64c6e6365be78c13c276ff4cba4",
    "size": 6000000,
    "status": "available"
  },
  "statusCode": 201
}
```



<!--

This file contains the documentation for REST API routes that are implemented
at the app level.  Use h2 to describe routes to fit in the global table of
content.

XXX Consider moving the routes and the documentation to a package.

XXX Consider writing a script that automatically creates JSON based on real API
calls, as for other package (see nog-content).

-->

### Get Job Status

    GET /jobs/:jobId/status

**Request query params**

 - None

**Request body**

 - empty

**Response**

    Status: 200

```json
{
  "data": {
    "status": "completed"
  },
  "statusCode": 200
}
```

### Post Job Status

    POST /jobs/:jobId/status

**Request query params**

 - None

**Request body**

 - `retryId (Integer)`: Current retry ID of this job
 - `status (String)`: Either `'completed'`, `'running'` or `'failed'`
 - `reason (String)`: Optional, reason if status is `'failed'`

**Response**

    Status: 200

```json
{
  "data": {},
  "statusCode": 200
}
```

### Get Job Progress

    GET /jobs/:jobId/progress

**Request query params**

 - None

**Request body**

 - empty

**Response**

    Status: 200

```json
{
  "data": {
    "progress": {
      "completed": 1,
      "percent": 50,
      "total": 2
    }
  },
  "statusCode": 200
}
```

### Post Job Progress

    POST /jobs/:jobId/progress

**Request query params**

 - None

**Request body**

 - `retryId (Integer)`: Current retry ID of this job
 - `progress (Object)`: with
    - `completed (Integer)`: Number of completed tasks
    - `total (Integer)`: Number of total tasks

**Response**

    Status: 200

```json
{
  "data": {},
  "statusCode": 200
}
```

### Post Job Log

    POST /jobs/:jobId/log

**Request query params**

 - None

**Request body**

 - `retryId (Integer)`: Current retry ID of this job
 - `message (String)`: Message to log
 - `level (integer)`: Optional, verbose level

**Response**

    Status: 200

```json
{
  "data": {},
  "statusCode": 200
}
```
