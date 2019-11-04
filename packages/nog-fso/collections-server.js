import { Mongo } from 'meteor/mongo';
import {
  KeyFilesSummary,
  KeyFsoId,
  KeyGitlabHost,
  KeyGitlabPath,
  KeyGitlabProjectId,
  KeyGitNogCommit,
  KeyId,
  KeyMeta,
  KeyName,
  KeyReadme,
  KeyRegistryId,
  KeyVid,
  makeCollName,
} from './collections.js';

// `optGitNogRegdOnly` completely disables read access via `nogfsog2nd`.
// All reads happen via `nogfsoregd` to `nogfsostad`.  See details below.
const optGitNogRegdOnly = true;

class FsoRegistry {
  constructor(doc) {
    const d = { ...doc };
    if (d[KeyVid]) {
      d[KeyVid] = Buffer.from(d[KeyVid]);
    }
    this.d = d;
  }

  id() { return this.d[KeyId]; }
  vid() { return this.d[KeyVid]; }
  name() { return this.d[KeyName]; }
}

class FsoRepo {
  constructor(doc) {
    const d = { ...doc };
    if (d[KeyVid]) {
      d[KeyVid] = Buffer.from(d[KeyVid]);
    }
    if (d[KeyFsoId]) {
      d[KeyFsoId] = Buffer.from(d[KeyFsoId]);
    }
    this.d = d;
  }

  id() { return this.d[KeyId]; }
  fsoId() { return this.d[KeyFsoId]; }
  path() { return this.d[KeyName]; }
  vid() { return this.d[KeyVid]; }
  registryId() { return this.d[KeyRegistryId]; }
  gitlabHost() { return this.d[KeyGitlabHost]; }
  gitlabPath() { return this.d[KeyGitlabPath]; }
  gitlabProjectId() { return this.d[KeyGitlabProjectId]; }

  gitNogCommit() { return this.d[KeyGitNogCommit]; }
  gitNogCommitId() { return this.d[KeyGitNogCommit].id; }
  statCommitId() { return this.d[KeyGitNogCommit].statCommitId; }
  contentCommitId() { return this.d[KeyGitNogCommit].contentCommitId; }
  metaCommitId() { return this.d[KeyGitNogCommit].metaCommitId; }

  hasMeta() { return this.d[KeyMeta] !== undefined; }
  hasReadmeIsUpdating() {
    return this.d[KeyReadme] && this.d[`${KeyReadme}.isUpdating`];
  }
  hasFilesSummaryIsUpdating() {
    return this.d[KeyFilesSummary] && this.d[`${KeyFilesSummary}.isUpdating`];
  }

  isShadowOnly() {
    return this.d[KeyGitlabHost] === '';
  }

  hasGitlabRepo() {
    const gh = this.d[KeyGitlabHost];
    return typeof gh === 'string' && gh !== '';
  }

  // `whichGitNogRead()` determines which GitNog endpoint is used for reading.
  // `whichGitNogWrite()` determines which one is used for writing.
  //
  // We decided to always write via `nogfsoregd` to `nogfsostad`.  See NOE-13.
  //
  // The config `optGitNogRegdOnly==true` specifies that all reads happens via
  // `nogfsoregd`, too.  `nogfsog2nd` is completly unused.
  //
  // An alternative read policy could be to use Gitlab via `nogfsog2nd` if a
  // Gitlab repo is configured and use `nogfsoregd` otherwise.  We keep the
  // alternative code path to illustrate the approach but mark it as
  // deprecated.  We will probably remove the deprecated code sooner than
  // later.
  whichGitNogRead() {
    if (optGitNogRegdOnly) {
      return 'regd';
    }
    // DEPRECATED: Read via `nogfsog2nd` is not actively used.
    if (this.isShadowOnly()) {
      return 'regd';
    }
    if (this.hasGitlabRepo()) {
      return 'g2nd';
    }
    return 'unknown';
  }
  whichGitNogWrite() { // eslint-disable-line class-methods-use-this
    return 'regd';
  }
}

function createCollectionsServer({ namespace }) {
  const regN = makeCollName(namespace, 'registries');
  const registries = new Mongo.Collection(regN, {
    transform: doc => new FsoRegistry(doc),
  });
  registries.rawCollection().createIndex({ [KeyName]: 1 }, { unique: true });

  const repos = new Mongo.Collection(makeCollName(namespace, 'repos'), {
    transform: doc => new FsoRepo(doc),
  });
  repos.rawCollection().createIndex({ [KeyName]: 1 }, { unique: true });
  repos.rawCollection().createIndex({ [KeyFsoId]: 1 }, { unique: true });

  return {
    registries,
    repos,
  };
}

export {
  createCollectionsServer,
};
