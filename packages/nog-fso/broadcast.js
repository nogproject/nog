import { Writable } from 'stream';
import { Meteor } from 'meteor/meteor';
import { Random } from 'meteor/random';
import {
  EV_BC_FSO_GIT_REF_UPDATED,
  grpc,
} from './proto.js';

function log(msg, ...args) {
  console.log(`[fso] ${msg}`, ...args);
}

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

const reconnectDelayS = 10;

function connect({
  registryName, conn, sysCallCreds, onEvent,
}) {
  const client = conn.broadcastClient(sysCallCreds);

  const observer = {
    registryName,
    isStopped: false,
    tail: null,
    stream: null,
    retryId: null,

    connect() {
      if (this.isStopped) {
        return;
      }

      const req = { channel: 'all', watch: true };
      if (this.tail) {
        req.after = this.tail;
      } else {
        req.afterNow = true;
      }
      const stream = client.events(req);

      // Do not use `stream.on.data`.  Instead pipe the stream, so that
      // response messages are processed in serial.  `on.data` would be called
      // in parallel with messages as they arrive.
      const cbStream = new Writable({
        objectMode: true,
        write: Meteor.bindEnvironment((rsp, enc, next) => {
          for (const ev of rsp.events) {
            try {
              onEvent(ev);
            } catch (err) {
              logerr(
                'Ignored broadcast onEvent() error.',
                'err', err.message,
              );
            }
            this.tail = ev.id;
          }
          next();
        }),
      });

      stream.on('error', Meteor.bindEnvironment((err) => {
        cbStream.destroy();
        // Ignore cancelled during `stop()`.
        if (err.code === grpc.status.CANCELLED && this.isStopped) {
          return;
        }
        log(
          'Fso broadcast events failed.',
          'err', err.message,
          'fsoreg', this.registryName,
        );
        this.deferConnect();
      }));

      stream.on('end', Meteor.bindEnvironment(() => {
        log(
          'Fso broadcast event stream ended unexpectedly.',
          'fsoreg', this.registryName,
        );
        this.deferConnect();
      }));

      this.stream = stream;
      stream.pipe(cbStream);

      log(
        'Connected to broadcast.',
        'fsoreg', this.registryName,
      );
    },

    deferConnect() {
      if (this.isStopped) {
        return;
      }

      this.retryId = Meteor.setTimeout(() => {
        this.retryId = null;
        log(
          'Reconnecting to broadcast.',
          'fsoreg', this.registryName,
        );
        this.connect();
      }, reconnectDelayS * 1000);

      log(
        'Scheduled reconnect to broadcast.',
        'delay', `${reconnectDelayS}s`,
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

  observer.connect();
  return observer;
}

function createBroadcast({ registryConns, sysCallCreds }) {
  return {
    observers: null,
    subscriptions: new Map(),

    start() {
      const obs = registryConns.map(({ registry, conn }) => connect({
        registryName: registry,
        conn,
        sysCallCreds,
        onEvent: ev => this.deliver(ev),
      }));
      this.observers = obs;
    },

    stop() {
      this.observers.forEach(o => o.stop());
    },

    deliver(ev) {
      for (const cb of this.subscriptions.values()) {
        cb(ev);
      }
    },

    subscribeGitRefUpdated(repoId, cb) {
      const id = Random.id();
      this.subscriptions.set(id, (ev) => {
        if (ev.event !== EV_BC_FSO_GIT_REF_UPDATED) {
          return;
        }

        if (!ev.bcChange.entityId.equals(repoId)) {
          return;
        }

        try {
          cb({ ref: ev.bcChange.gitRef, commit: ev.bcChange.gitCommit });
        } catch (err) {
          logerr(
            'Ignored subscribeGitRefUpdated() callback error.',
            'err', err.message,
          );
        }
      });
      return id;
    },

    subscribeGitRefUpdatedAll(cb) {
      const id = Random.id();
      this.subscriptions.set(id, (ev) => {
        if (ev.event !== EV_BC_FSO_GIT_REF_UPDATED) {
          return;
        }

        try {
          cb({
            repoId: ev.bcChange.entityId,
            ref: ev.bcChange.gitRef,
            commit: ev.bcChange.gitCommit,
          });
        } catch (err) {
          logerr(
            'Ignored subscribeGitRefUpdatedAll() callback error.',
            'err', err.message,
          );
        }
      });
      return id;
    },

    unsubscribe(id) {
      this.subscriptions.delete(id);
    },
  };
}

export {
  createBroadcast,
};
