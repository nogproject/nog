# Exploring the REST API

Programs can interact with Nog through a REST API.  This tutorial shows
how the low level API can be explored from the command line.

<!-- toc -->

## The Nog starter pack

The [Nog starter pack](/nog/packages/files/programs/nog-starter-pack/index!0)
contains files that are used in the tutorials.  Get the starter pack and unpack
it.  The starter pack archive is available at:

 - [nog-starter-pack/index!0/content.tar.xz](/nog/packages/files/programs/nog-starter-pack/index!0/content.tar.xz)

## Configuring the environment for the REST API

Create and download an API key from the [settings](/settings).  Configure the
key in your environment:

```bash
export NOG_KEYID=<keyid>
export NOG_SECRETKEY=<secretkey>
```

## Using curl to access the API

The REST API is documented in [apidoc](apidoc.md).

Requests are authenticated using a cryptographic signature scheme.
`bin/sign-req` from the starter pack creates the signature on the command line.

For example, sign the URL to get the master branch for repo `nog/doc`:

```bash
./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/refs/branches/master
```

Use the signed URL with curl to query the API:

```bash
curl $(./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/refs/branches/master)
```

Use Python's json.tool or `jq` to format the JSON for reading:

```bash
curl $(./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/refs/branches/master) | python -m json.tool
curl $(./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/refs/branches/master) | jq .
```

You can follow the hrefs in the JSON response to navigate the repo content.
Just copy and paste the href from the previous JSON response to build the next
command.

Get the commit by following `data.entry.href`:

```bash
curl $(./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/commits/<sha1>) | python -m json.tool
```

Get the tree by following `data.tree.href`:

```bash
curl $(./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/trees/<sha1>) | python -m json.tool
```

Then follow the first of the entries (`data.entries[0].href`):

```bash
curl $(./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/objects/<sha1>) | python -m json.tool
```

It should output a JSON with the markdown text for the documentation index in
`data.meta.content`.

You can use a small inline Python program to parse the JSON and print the text
to the console:

```bash
curl $(./bin/sign-req GET https://nog.zib.de/api/v1/repos/nog/doc/db/objects/<sha1>) |
python -c 'import json; import sys; res = json.load(sys.stdin); print(res); print(res["data"]["meta"]["content"])'
```
