# nogpy -- HACKING

## How to start a new release cycle?

To start a new release cycle, add a section to the CHANGELOG and mark it as
'(unreleased)'.  Do not yet update the version in `package.json`.

We keep the old version in `package.json` to protect against accidentally
publishing a new version.  The CHANGELOG is considered sufficient indication
that there are changes since the last release.

## How to publish a release?

Use a p/ branch to prepare and publish a release.

Review the Git history and update the CHANGELOG in preparation for a release.

See `test_nog.py` for tests.

To publish a release, change the version in `package.json`, remove
'(unreleased)' from the CHANGELOG section, and commit with message
`nogpy-<version>: <highlights>`.  Polish the p/ branch.  Then publish from the
commit that changes the version:

```bash
export NOG_API_URL="https://nog.zib.de"
export NOG_USERNAME="nog"
...

nogpm publish
```

## How to test the release process during development?

```bash
export NOG_API_URL="https://nog.devspr..."
export NOG_USERNAME="<you>"
...

nogpm publish --registry ${NOG_USERNAME}/test-nogpackages --force
```
