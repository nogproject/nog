# Developing Nog App 2

## Introduction

Nog App 2 development started in 2019-03 in the sub-directory `web/`.  The
initial goal is a web application that uses Meteor 1.8 or newer with ECMAScript
and React for managing FSO users and access tokens.

Meteor packages and testing applications usually have a suffix `-2` if they are
similar to a previous variant that already existed at the end of 2018.
Examples:

```
web/packages/nog-error-2   packages/nog-error
web/apps/access-testapp-2  examples/access-testapp
```

Some instructions from [README](./README.md) and [HACKING](./HACKING.md) may
still be relevant.  They should eventually be replaced by updated instructions
in this document.

## Testing

Package tests run in `web/apps/packages-testapp`.  See example package
`nog-example-2`, in particular the scripts sections of:

```
web/packages/nog-example-2/package.json
web/apps/packages-testapp/package.json
```
