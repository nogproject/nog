# Package `nog-fso-authz`

## Introduction

Package `nog-fso-authz` provides mechanisms to manage FSO access settings.

See source `nog-fso-authz/fso-authz.js` for available rules.

See `fso-testapp` for an example.

## `nog-fso-authz.compileFsoPermissions()` (server)

`compileFsoPermissions()` translates access rules into access statements that
can be used with `NogAccess`.  Example:

```javascript
const statements = compileFsoPermissions(settings.permissions);
NogAccess.addStatements(statements);
```

## `nog-fso-authz.matchFsoPermissions()` (server)

`matchFsoPermissions()` can be used to validate access rule settings.  See
`fso-testapp` for an example.
