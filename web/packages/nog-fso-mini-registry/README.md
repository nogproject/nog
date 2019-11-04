# Package `nog-fso-mini-registry`

## Introduction

Package `nog-fso-mini-registry` implements a module that can be used to observe
Nogfsoregd and mirror changes to a repos Mongo collection that allows resolving
FSO repo IDs to paths.

See `apps/fso-testapp` for a complete example.

## `nog-fso-mini-registry.createFsoMiniRegistryModuleServer()` (server)

`createFsoMiniRegistryModuleServer()` returns a module instance.

```javascript
const NogFsoMiniRegistry = createFsoMiniRegistryModuleServer({
  namespace: NsFsoMiniRegistry,
  registryConns: registryConns,
  rpcSysCallCreds: sysCallCreds,
});
...
const obs1 = NogFsoMiniRegistry.observeRegistry(reg1);
const obs2 = NogFsoMiniRegistry.observeRegistry(reg2);
...
```
