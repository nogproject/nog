# Nog Load Tests
By spr
<!--@@VERSIONINC@@-->

## General architecture

Test state is stored in a MongoDB to support concurrent testing from several
servers.

The test scripts use a shared config from `NOG_LOAD_TEST_CONFIG`, which can
either contain JSON (first character must be `{`) or a path to a `.json` file.

## Test config

Example file `_private/<host>-config.json`:

```
{
    "url": "http://localhost:3000",
    "loadTestsMongoUrl": "mongodb://127.0.0.1:3001/load-test",
    "admin": {
        "username": "<admin-user>",
        "password": "<admin-password>"
    },
    "accountSpec": {
        "email": "prohaska+nogtestuser_#{username}@zib.de"
    },
    "downloadDirectory": "/tmp/nog-load-test-downloads",
    "nAccounts": 10
}
```

## Docker

`load-tests/image` contains a Docker image to run tests in a container.
See comments in the Dockerfile.

## Account tests

```bash
host=localhost
cd load-test
export NOG_LOAD_TEST_CONFIG=$(pwd)/_private/${host}-config.json
cd accounts
```

Create `config.nAccounts` sequentially:

```
nightwatch -e chrome
```

To create more account, run multiple times sequentially:

```
for i in {0..9}; do
    nightwatch -e chrome
done
```

Do not run in parallel, because we use a global download path.

Furthermore, Nightwatch prepares all tests before it starts executing.  The
timestamp for user names it determined during preparation time.  Better run
Nightwatch to spread the user timestamps and avoid problems due to many queued
tests.

## Useful command

Deleting test users in Mongo shell:

```
use meteor
db.users.remove({username: {$regex: /^tusr-/}})

use load-test
db.testUsers.remove({username: {$regex: /^tusr-/}})
```
