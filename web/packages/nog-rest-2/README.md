# Package `nog-rest-2`

## Introduction

Package `nog-rest-2` provides a mechanism to hook a REST API into Meteor's
`WebApp`.

See `fso-testapp` for full example.

Example:

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

## `nog-rest-2.createRestServer()` (server)

`createRestServer()` returns a REST API server:

```
const api = createRestServer({});
```

## `api.auth` (server)

`api.auth` is the hook for authentication middlewares.

## `api.useActions()` (server)

`api.useActions()` adds a list of actions that are specified by objects:

```javascript
{
  method: 'GET' | 'POST' | ...,
  path: '/foo/:bar',
  action(req) { ... },
}
```

`path` is a path-to-regexp pattern.

`req` has at least the following fields:

* `params`: the path params;
* `query`: the parsed query string;
* `baseUrl`: like Express <https://expressjs.com/en/api.html#req.baseUrl>.

If successful, `action()` returns a plain JavaScript object that is sent as
JSON in the HTTP response.

`action()` may throw an error, which is translated to an HTTP error response.

`action()` may return a special object to respond with a redirect:

```
{
  statusCode: 30x,
  location: '/new/path',
}
```

## `api.app` (server)

`api.app` is the middleware that is mounted into Meteor `WebApp`.  It may be
mounted multiple times:

```javascript
WebApp.connectHandlers.use('/api/v1/fso', api.app);
WebApp.connectHandlers.use('/api/fso', api.app);
```

`api.app` handles all paths.  If no action matches, it responds with 404 "Not
Found".
