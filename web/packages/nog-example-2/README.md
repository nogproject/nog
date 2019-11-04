# Package `nog-example-2`

## Introduction

Package `nog-example-2` is an example Meteor package for Nog App 2.

## Default module

### `nog-example-2.sum()` (anywhere)

`sum(a, b)` returns `a + b` as a `Number`.

### `nog-example-2.serverSum()` (server)

`serverSum(a, b)` returns `a + b` as a string prefixed by `server:`.

### `nog-example-2.clientSum()` (client)

`clientSum(a, b)` returns `a + b` as a string prefixed by `client:`.

## Dependency injection

### `nog-example-2.createExampleModuleServer()` (server)

`createExampleModuleServer()` instantiates a module with dependency injection:

```javascript
NogExample = createExampleModuleServer({ serverName: 'foo' };
```

### `nog-example-2.createExampleModuleClient()` (client)

`createExampleModuleClient()` instantiates a module with dependency injection:

```javascript
NogExample = createExampleModuleClient({ clientName: 'foo' };
```

### `NogExample.sum()` (anywhere)

`NogExample.sum(a, b)` returns `a + b` as a `Number`.

### `NogExample.serverSum()` (server)

`serverSum(a, b)` returns `a + b` as a string prefixed by `server
${serverName}:`.

### `NogExample.clientSum()` (client)

`clientSum(a, b)` returns `a + b` as a string prefixed by `client
${clientName}:`.
