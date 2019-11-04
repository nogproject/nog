/* eslint-disable camelcase */
import { Mongo } from 'meteor/mongo';
import {
  KeyErrorMessage,
  KeyFilesSummary,
  KeyFsoId,
  KeyGitNogCommit,
  KeyGitlabProjectId,
  KeyGitlabUrl,
  KeyId,
  KeyMetadata,
  KeyName,
  KeyReadme,
  KeyRefreshContentRequested,
  KeyStatRequested,
  KeyStatStatus,
  makeCollName,
} from './collections.js';

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

function arrayToHex(a) {
  const pad0 = s => (`0${s}`).slice(-2);
  const byteToHex = v => pad0(v.toString(16));
  return a.reduce((hex, val) => hex + byteToHex(val), '');
}

function arrayToUuidString(a) {
  const p0 = arrayToHex(a.slice(0, 4));
  const p4 = arrayToHex(a.slice(4, 6));
  const p6 = arrayToHex(a.slice(6, 8));
  const p8 = arrayToHex(a.slice(8, 10));
  const p10 = arrayToHex(a.slice(10, 16));
  return `${p0}-${p4}-${p6}-${p8}-${p10}`;
}

class FsoRepoC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  fsoIdString() { return arrayToUuidString(this.d[KeyFsoId]); }
  name() { return this.d[KeyName]; }
  path() { return this.d[KeyName]; }

  gitlabUrl() { return this.d[KeyGitlabUrl]; }
  gitlabProjectId() { return this.d[KeyGitlabProjectId]; }

  statStatus() { return this.d[KeyStatStatus]; }
  statStatusIsClean() {
    const st = this.statStatus();
    if (!st) {
      return true;
    }
    return (st.nNew === 0 && st.nModified === 0 && st.nDeleted === 0);
  }

  // The xExists functions are used in the GUI components to skip rendering
  // fragments if they would access undefined values.
  gitNogCommitExists() { return this.d[KeyGitNogCommit] !== undefined; }
  metadataExists() { return this.d[KeyMetadata] !== undefined; }
  readmeExists() { return this.d[KeyReadme] !== undefined; }
  filesSummaryExists() { return this.d[KeyFilesSummary] !== undefined; }
  statStatusExists() { return this.d[KeyStatStatus] !== undefined; }

  gitNogCommitId() { return this.d[KeyGitNogCommit].id; }

  statAuthor() {
    const { statAuthorName: n, statAuthorEmail: e } = this.d[KeyGitNogCommit];
    if (!n || !e) {
      logerr('Invalid stat commit author');
      return null;
    }
    return `${n} <${e}>`;
  }
  statDate() {
    if (!this.d[KeyGitNogCommit] || !this.d[KeyGitNogCommit].statDate) {
      logerr('stat: Invalid date');
      return null;
    }
    return new Date(this.d[KeyGitNogCommit].statDate);
  }

  shaAuthor() {
    const { shaAuthorName: n, shaAuthorEmail: e } = this.d[KeyGitNogCommit];
    if (!n || !e) {
      logerr('Invalid sha commit author');
      return null;
    }
    return `${n} <${e}>`;
  }
  shaDate() {
    if (!this.d[KeyGitNogCommit] || !this.d[KeyGitNogCommit].shaDate) {
      logerr('Invalid sha commit date');
      return null;
    }
    return new Date(this.d[KeyGitNogCommit].shaDate);
  }

  contentAuthor() {
    const {
      contentAuthorName: n, contentAuthorEmail: e,
    } = this.d[KeyGitNogCommit];
    if (!n || !e) {
      logerr('Invalid content commit author');
      return null;
    }
    return `${n} <${e}>`;
  }
  contentDate() {
    if (!this.d[KeyGitNogCommit] || !this.d[KeyGitNogCommit].contentDate) {
      logerr('Invalid content commit date');
      return null;
    }
    return new Date(this.d[KeyGitNogCommit].contentDate);
  }

  metaAuthor() {
    const { metaAuthorName: n, metaAuthorEmail: e } = this.d[KeyGitNogCommit];
    if (!n || !e) {
      logerr('Invalid meta commit author');
      return null;
    }
    return `${n} <${e}>`;
  }
  metaDate() {
    if (!this.d[KeyGitNogCommit] || !this.d[KeyGitNogCommit].metaDate) {
      logerr('Invalid meta commit date');
      return null;
    }
    return new Date(this.d[KeyGitNogCommit].metaDate);
  }
  metaCommitId() {
    return this.d[KeyGitNogCommit].metaCommitId;
  }

  filesSummary() { return this.d[KeyFilesSummary]; }

  readme() { return this.d[KeyReadme].text; }

  statRequestTime() { return this.d[KeyStatRequested]; }
  refreshContentRequestTime() { return this.d[KeyRefreshContentRequested]; }

  errorMessage() { return this.d[KeyErrorMessage]; }

  metaIsUpdating() {
    return this.d[KeyMetadata].isUpdating;
  }
  metadata() {
    const m = this.d[KeyMetadata].kvs;
    if (!m) {
      return new Map();
    }
    return new Map(m.map(kv => [kv.k, kv.v]));
  }
  meta() {
    return {
      author: this.metaAuthor(),
      date: this.metaDate(),
      commitId: this.metaCommitId(),
      isUpdating: this.metaIsUpdating(),
      data: this.metadata(),
    };
  }
}

function createCollectionsClient({ namespace }) {
  const repos = new Mongo.Collection(makeCollName(namespace, 'repos'), {
    transform: doc => new FsoRepoC(doc),
  });
  return {
    repos,
  };
}

export {
  createCollectionsClient,
  KeyName,
};
