import { Writable } from 'stream';
import { Meteor } from 'meteor/meteor';
import { Promise } from 'meteor/promise';
import { NogError } from 'meteor/nog-error';
const {
  nogthrow,
} = NogError;

import { createProcessor } from './process.js';
import { grpc } from './proto.js';

import {
  KeyId,
  KeyVid,
  KeyName,
} from './collections.js';

import {
  ERR_FSO,
} from './errors.js';

const delayS = 10;

function log(msg, ...args) {
  console.log(`[fso] ${msg}`, ...args);
}

function observeRegistry({
  conn, registries, repos, registryName, sysCallCreds,
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

  const regClient = conn.registryClient(sysCallCreds);
  const proc = createProcessor({
    conn, sysCallCreds, repos, registryId,
  });

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

      // Do not use `stream.on.data`.  Instead pipe the stream, so that
      // response messages are processed in serial.  `on.data` would be called
      // in parallel with messages as they arrive.
      const cbStream = new Writable({
        objectMode: true,
        write: Meteor.bindEnvironment((rsp, enc, next) => {
          for (const ev of rsp.events) {
            try {
              Promise.await(proc.applyEvent(ev));
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
          'Fso registry events failed.',
          'err', err.message,
          'fsoreg', this.registryName,
        );
        this.deferConnect();
      }));

      stream.on('end', Meteor.bindEnvironment(() => {
        log(
          'Fso registry event stream unexpectedly ended.',
          'fsoreg', this.registryName,
        );
        this.deferConnect();
      }));

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

  observer.connect();
  return observer;
}

export {
  observeRegistry,
};
