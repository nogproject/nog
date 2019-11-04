# Package `nog-cluster-2`

## Introduction

`NogCluster.IdPartition` together with `NogCluster.registerHeartbeat()`
implements a basic work distribution scheme for a cluster of application
instances.

See `fso-testapp` for a full example.

Example:

```javascript
parser.defSetting(clusterOptSingleInstanceModeSetting);

const NogCluster = createClusterModuleServer({
  namespace: NsCluster,
  optSingleInstanceMode: Meteor.settings.cluster.optSingleInstanceMode,
  optGlobalReadOnly: Meteor.settings.optGlobalReadOnly,
});

const partition = new NogCluster.IdPartition({ name: 'minifso', max: 2 });
partition.onacquire = (part) => {
  startPart(part);
};
partition.onrelease = (part) => {
  stopPart(part);
};
NogCluster.registerHeartbeat(partition);
```
