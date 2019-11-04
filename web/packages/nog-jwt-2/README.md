# Package `nog-jwt-2`

## Introduction

Package `nog-jwt-2` implements JWT-based authorization for FSO.

See `apps/fso-testapp` for complete examples.

Example how to use a JWT token to connect with gRPC to Nogfsoregd:

```javascript
const rpcTokenProvider = createFsoTokenProvider({
  issuer,
  cert: jwtCertFile,
  domains: [],
  users: fakeUsers,
});
const sysCallCreds = createAuthorizationCallCreds(
  rpcTokenProvider.fsoSysToken,
  { username: 'nog-app-2' },
  {
    subuser: 'minifso',
    scopes: [
      { action: AA_FSO_READ_REGISTRY, names: ['exreg'] },
    ],
  },
);
const conn = connectFsoRegGrpc({ addr, certFile, caFile });
```

Example how to expose a REST API to issue JWTs:

```javascript
const api = createRestServer({});
const authn = createBearerJwtAuthn({
  users: Meteor.users,
  issuer: settings.jwt.issuer,
  audience: settings.jwt.issuer,
  ou: settings.jwt.ou,
  ca: settings.jwt.ca,
  testingJtis: settings.jwt.testingJtis || [],
});
api.auth.use(authn.middleware);
api.useActions(createAuthApiActions({
  checkAccess: NogAccess.checkAccess,
  testAccess: NogAccess.testAccess,
  repoResolver: NogFsoMiniRegistry.repoResolver,
  rpcTokenProvider,
}));
WebApp.connectHandlers.use('/api/v1/fso', api.app);
WebApp.connectHandlers.use('/api/fso', api.app);
```

## `nog-jwt-2.createAuthApiActions()` (server)

`createAuthApiActions()` returns a list of API handlers that can be used with
package `nog-rest-2`.  See examples.

## `nog-jwt-2.createBearerJwtAuthn()` (server)

`createBearerJwtAuthn()` returns an authentication middleware that can be used
with package `nog-rest-2`.  See examples.

## `nog-jwt-2.createFsoTokenProvider()` (server)

`createFsoTokenProvider()` returns a JWT provider that can be used for gRPCs
with `nog-fso-grpc` or to implement API handlers with `createAuthApiActions()`.
See examples.
