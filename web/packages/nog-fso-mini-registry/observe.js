import { Writable } from 'stream';
import { Meteor } from 'meteor/meteor';
import {
  EV_FSO_REPO_ACCEPTED,
  grpc,
} from 'meteor/nog-fso-grpc';

import {
  nogthrow,
  ERR_FSO,
} from './errors.js';

import {
  KeyId,
  KeyVid,
  KeyName,
  KeyFsoId,
  KeyPath,
  KeyRegistryId,
} from './collections.js';

const delayS = 10;

function isDuplicateMongoIdError(err) {
  return err.code === 11000;
}

function createProcessor({
  log, repos, registryId,
}) {
  return {
    repos,
    registryId,

    applyEvent(ev) {
      switch (ev.event) {
        case EV_FSO_REPO_ACCEPTED:
          this.applyRepoAccepted(ev);
          break;

        // Silently ignore other events.
        default:
          break;
      }
    },

    applyRepoAccepted(ev) {
      const repoFsoId = ev.fsoRepoInfo.id;
      const path = ev.fsoRepoInfo.globalPath;
      this.insertRepo({ repoFsoId, path });
      log(
        'Applied fso registry event:',
        'event', ev,
      );
    },

    insertRepo({ repoFsoId, path }) {
      try {
        const id = this.repos.insert({
          [KeyFsoId]: repoFsoId,
          [KeyPath]: path,
          [KeyRegistryId]: this.registryId,
        });
        log(
          'Inserted repo.',
          'path', path,
          'id', id,
        );
      } catch (err) {
        if (isDuplicateMongoIdError(err)) {
          return;
        }
        throw err;
      }
    },
  };
}

function createObserver({
  log, conn, registries, repos, registryName, rpcSysCallCreds,
  testingOnWillBlock,
}) {
  registries.upsert(
    { [KeyName]: registryName },
    { $set: { [KeyName]: registryName } },
  );
  const registry = registries.findOne({ [KeyName]: registryName });
  if (!registry) {
    nogthrow(ERR_FSO, { reason: 'Failed to find registry doc.' });
  }
  const registryId = registry.id();

  const regClient = conn.registryClient(rpcSysCallCreds);
  const proc = createProcessor({ log, repos, registryId });

  const observer = {
    registryId,
    registryName,
    stream: null,
    isStopped: false,
    retryId: null,
    tail: null,

    updateTail(vid) {
      const sel = { [KeyId]: this.registryId };
      if (this.tail) {
        sel[KeyVid] = this.tail;
      } else {
        sel[KeyVid] = { $exists: false };
      }
      const n = registries.update(sel, { $set: { [KeyVid]: vid } });
      if (n !== 1) {
        nogthrow(ERR_FSO, {
          reason: 'Failed to save registry tail vid.',
        });
      }

      this.tail = vid;
    },

    connect() {
      if (this.isStopped) {
        return;
      }

      try {
        const info = regClient.infoSync({ registry: this.registryName });
        log(
          'Connected to fso registry.',
          'fsoreg', this.registryName,
          'vid', info.vid.toString('hex'),
        );

        const reg = registries.findOne(this.registryId);
        if (!reg) {
          nogthrow(ERR_FSO, {
            reason: 'Failed to find registry doc.',
          });
        }
        this.tail = reg.vid();
      } catch (err) {
        log(
          'Failed to prepare connecting to fso registry events.',
          'fsoreg', this.registryName,
          'err', err,
        );
        this.deferConnect();
        return;
      }

      const req = { registry: this.registryName, watch: true };
      if (this.tail) {
        req.after = this.tail;
        log(
          'Begin processing events after tail.',
          'after', this.tail.toString('hex'),
        );
      } else {
        log('Begin processing from event epoch.');
      }
      const stream = regClient.events(req);

      // Do not use `stream.on.data`, but pipe the stream, so that response
      // messages are processed in serial.  `on.data` would be called in
      // parallel with messages as they arrive.
      const cbStream = new Writable({
        objectMode: true,
        write: async (rsp, enc, next) => {
          for (const ev of rsp.events) {
            try {
              proc.applyEvent(ev);
              this.updateTail(ev.id);
            } catch (err) {
              this.stream.cancel();
              this.stream = null;
              // Do not call `next(err)`, since the stream is already canceled.
              // `on.error` schedules reconnect.
              log(
                'Failed to apply event; cancelled stream.',
                'err', err.message,
                'fsoreg', this.registryName,
              );
              return;
            }
          }
          if (testingOnWillBlock && rsp.willBlock) {
            testingOnWillBlock({
              registry: this.registryName,
            });
          }
          next();
        },
      });

      stream.on('error', async (err) => {
        cbStream.destroy();
        // Ignore cancelled during `stop()`.
        if (err.code === grpc.status.CANCELLED && this.isStopped) {
          return;
        }
        log(
          'Fso registry events failed.',
          'err', err.message,
          'fsoreg', this.registryName,
        );
        this.deferConnect();
      });

      stream.on('end', async () => {
        log(
          'Fso registry event stream unexpectedly ended.',
          'fsoreg', this.registryName,
        );
        this.deferConnect();
      });

      this.stream = stream;
      stream.pipe(cbStream);
    },

    deferConnect() {
      if (this.isStopped) {
        return;
      }

      this.retryId = Meteor.setTimeout(() => {
        this.retryId = null;
        log(
          'Reconnecting to fso registry.',
          'fsoreg', this.registryName,
        );
        this.connect();
      }, delayS * 1000);

      log(
        'Scheduled reconnect.',
        'delay', `${delayS}s`,
        'fsoreg', this.registryName,
      );
    },

    stop() {
      this.isStopped = true;

      if (this.retryId) {
        Meteor.clearTimeout(this.retryId);
        this.retryId = null;
      }

      if (this.stream) {
        this.stream.cancel();
        this.stream = null;
      }
    },
  };

  return observer;
}

function createRegistryObserverManager({
  log, registryConns, registries, repos, rpcSysCallCreds,
}) {
  const conns = new Map();
  registryConns.forEach(({ registry, conn }) => {
    conns.set(registry, conn);
  });

  function observeRegistry(registryName, { testingOnWillBlock } = {}) {
    const conn = conns.get(registryName);
    if (!conn) {
      nogthrow(ERR_FSO, { reason: 'Unknown registry.' });
    }
    const observer = createObserver({
      conn, registries, repos, registryName, rpcSysCallCreds, log,
      testingOnWillBlock,
    });
    observer.connect();
    return observer;
  }

  const manager = {
    observeRegistry,
  };
  return manager;
}

export {
  createRegistryObserverManager,
};
