// @flow

import { _ } from 'meteor/underscore';
import { check, Match } from 'meteor/check';


type RemoteSettings = {
  name: string,
  url: string,
  username: ?string,
  keyid: ?string,
  secretkey: ?string,
  namespace: ?{ meth: string, coll: string },
}

const matchRemoteSettings = {
  name: String,
  url: String,
  username: Match.Maybe(String),
  keyid: Match.Maybe(String),
  secretkey: Match.Maybe(String),
  namespace: Match.Maybe({ meth: String, coll: String }),
};

type CachingSettings = {
  maxNElements: number,
  maxAge_s: number,
}

const matchCachingSettings = {
  maxNElements: Number,
  maxAge_s: Number,
};

type SyncSettings = {
  peers: string[],
  us: string,
  interval_ms: ?number,
  fallbackPullInterval_ms: ?number,
  remotes: ?RemoteSettings[],
  afterMergeWait: ?{ min_ms: number, max_ms: number },
  afterSnapWait: ?{ min_ms: number, max_ms: number },
};

const matchSyncSettings = {
  peers: [String],
  us: String,
  interval_ms: Match.Maybe(Number),
  fallbackPullInterval_ms: Match.Maybe(Number),
  remotes: Match.Maybe([matchRemoteSettings]),
  afterMergeWait: Match.Maybe({ min_ms: Number, max_ms: Number }),
  afterSnapWait: Match.Maybe({ min_ms: Number, max_ms: Number }),
};

type RemoteConfig = {
  name: string,
  url: string,
  username: string,
  keyid: ?string,
  secretkey: ?string,
  namespace: ?{ meth: string, coll: string },
}

type CachingConfig = CachingSettings;

type NogSyncConfig = {
  peers: string[],
  us: ?string,
  interval_ms: number,
  fallbackPullInterval_ms: number,
  afterMergeWait: { min_ms: number, max_ms: number },
  afterSnapWait: { min_ms: number, max_ms: number },
  remotes: RemoteConfig[],
  caching: CachingConfig,
  configure: () => void,
  peerUsername: (peerName: ?string) => string,
  ourUsername: () => string,
  ourPeerName: () => ?string,
};

type createNogSyncConfig$opts = {
  settings: {
    sync: mixed,
    caching: ?{
      sync: mixed,
    },
  },
};

function createNogSyncConfig(
  { settings }: createNogSyncConfig$opts,
) : NogSyncConfig {
  // After settings pass check, typecast to to get flowtype checking for the
  // remainder of the function.
  check(
    settings, Match.ObjectIncluding({
      sync: Match.Maybe(matchSyncSettings),
      caching: Match.Maybe({
        sync: Match.Maybe(matchCachingSettings),
      }),
    })
  );
  const sync: ?SyncSettings = (settings.sync: any);
  let caching: ?CachingSettings;
  if (settings.caching != null) {
    caching = ((settings.caching.sync: any): ?CachingSettings);
  }

  let peers = [];
  let us = null;
  // eslint-disable-next-line camelcase
  let interval_ms = 10 * 1000;
  // eslint-disable-next-line camelcase
  let fallbackPullInterval_ms = 10 * 1000;
  // eslint-disable-next-line camelcase
  let afterMergeWait = { min_ms: 100, max_ms: 5000 };
  // eslint-disable-next-line camelcase
  let afterSnapWait = { min_ms: 200, max_ms: 1000 };
  let remotes = [];

  function peerUsername(peer: ?string) {
    return peer != null ? `nogsyncbot${peer}` : 'nobody';
  }

  if (sync != null) {
    peers = sync.peers;
    us = sync.us;
    if (sync.interval_ms != null) {
      // eslint-disable-next-line camelcase
      interval_ms = sync.interval_ms;
    }
    if (sync.fallbackPullInterval_ms != null) {
      // eslint-disable-next-line camelcase
      fallbackPullInterval_ms = sync.fallbackPullInterval_ms;
    }
    if (sync.afterMergeWait != null) {
      afterMergeWait = sync.afterMergeWait;
    }
    if (sync.afterSnapWait != null) {
      afterSnapWait = sync.afterSnapWait;
    }
    if (sync.remotes != null) {
      remotes = sync.remotes.map((r) => {
        const s = _.clone(r);
        if (s.username == null) {
          s.username = peerUsername(s.name);
        }
        return s;
      });
    }
  }

  if (caching == null) {
    caching = {
      maxNElements: (32 * 1024),
      maxAge_s: (10 * 60),  // eslint-disable-line camelcase
    };
  }

  return {
    peers, us, interval_ms, fallbackPullInterval_ms, afterMergeWait,
    afterSnapWait, caching, remotes,

    configure() {
      console.log('configure() not yet implemented');
    },

    peerUsername,

    ourUsername() {
      return this.peerUsername(this.us);
    },

    ourPeerName() {
      return this.us;
    },
  };
}


export { createNogSyncConfig };
