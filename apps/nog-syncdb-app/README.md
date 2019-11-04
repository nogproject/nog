# Syncdb app

Usage:

```
cd apps/nog-syncdb-app/meteor
meteor run --port 5000 --settings _private/syncdb-settings.json
```

`nog-syncdb-app` synchronizes a `nog-app` deployment to another MongoDB.  It
can be used to migrate to a new database or to maintain a standby database.

A synchronization starts with a full copy of relevant collections, followed by
oplog tailing to incrementally synchronize changes.

Syncdb is controlled by a settings file with the following format:

```
{
  "syncdb": {
    "stateId": "...",
    "optForceFullCopy": false,
    "waitBeforeCopy_s": 30,
    "src": {
      "url": "mongodb://<user>...@<host>.../<srcdb>?replicaSet=<srcRsId>",
      "dbns": "<srcdb>",
      "oplogurl": "mongodb://<user>...@<host>.../local?authSource=<srcdb>"
    },
    "dst": {
      "url": "mongodb://<user>...@<host>.../<dstdb>?replicaSet=<dstRsId>"
    }
  }
}
```

`stateId` must uniquely identify the sync job.  It is used to store the sync
state into the app's local db to restart oplog tailing from the right position
after an app restart.

`optForceFullCopy` can be set to true to force a full copy.  The default is to
start a full copy only if no sync state is found in the app's local db.

`waitBeforeCopy_s` is a safety delay before starting a full copy.

`src.url` is the source db that will be copied.  The db user must have read
access.  `src.dbns` is the MongoDB database namespace, which is identical to
the database name used in `src.url`.  `src.oplogurl` is the oplog that contains
the source database ops.  The db user must have oplog access.

`dst.url` is the destination db.  The db user must have write access.
