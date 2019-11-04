# Package `nog-fso-grpc`

## Introduction

Package `nog-fso-grpc` exports gRPC stubs for the Protocol Buffers that are
defined in the backend package `nogfsopb`.  We export more Protocol Buffer
symbols as needed.  See source `nog-fso-grpc/proto.js` and
`nog-fso-grpc/server-index.js`.

## `nog-fso-grpc.connectFsoRegGrpc()` (server)

`connectFsoRegGrpc()` opens a connection to a Nogfsoregd server.  The returned
connection object has functions to create gRPC clients that provide synchronous
stubs for use in Meteor.  We wrap more stubs as needed.  See source
`nog-fso-grpc/grpc.js` for details.

Example:

```javascript
const conn = connectFsoRegGrpc({
  addr: 'fsoreg.example.com:7550',
  certFile: '/path/to/combined.pem',
  caFile: '/path/to/ca.pem',
}));

const sysCallCreds = createAuthorizationCallCreds(
  rpcTokenProvider.fsoSysToken, // See package `nog-jwt-2`.
  { username: 'fso-testapp' },
  {
    subuser: 'minifso',
    scopes: [
      { action: AA_FSO_READ_REGISTRY, names: [registryExreg] },
    ],
  },
);

const regd = conn.registryClient(sysCallCreds);
const repos = regd.getReposSync({ registry: 'exreg' });
```

## `nog-fso-grpc.createAuthorizationCallCreds()` (server)

`createAuthorizationCallCreds(rpcAuthorization, euid, opts)` returns gRPC
`CallCredentials` that manage a JWT that is issued by `rpcAuthorization(euid,
...)` in the GRPC metadata `authorization`.  The JWT validity duration and
refresh period can be specified in `opts: { expiresInS, refreshPeriodS }`.

See example at `connectFsoRegGrpc()`.
