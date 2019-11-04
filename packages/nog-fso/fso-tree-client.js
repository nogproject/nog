import moment from 'moment';
import { Mongo } from 'meteor/mongo';
import { check, Match } from 'meteor/check';
import {
  CollNameContent,
  CollNameFiles,
  CollNameTreeErrors,
  KeyContent,
  KeyId,
  KeyMeta,
  KeyRepoName,
  KeyStatInfo,
  KeyTreeErrorMessage,
  KeyTreePath,
  PubNameTree,
  PubNameTreePathContent,
} from './fso-tree.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';

function arrayToHex(a) {
  const pad0 = s => (`0${s}`).slice(-2);
  const byteToHex = v => pad0(v.toString(16));
  return a.reduce((hex, val) => hex + byteToHex(val), '');
}

const ModeBits = {
  Type: 0o170000, // type mask
  Dir: 0o040000,
  Regular: 0o100000,
  Symlink: 0o120000,
  Gitlink: 0o160000,
};

class FsoFileC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  path() { return this.d[KeyTreePath]; }
  stat() { return this.d[KeyStatInfo]; }
  meta() { return this.d[KeyMeta]; }
  mode() { return this.stat().mode; }
  /* eslint-disable no-bitwise */
  typeBits() { return this.mode() & ModeBits.Type; }
  /* eslint-enable no-bitwise */
  isDir() { return this.typeBits() === ModeBits.Dir; }
  isNogbundle() {
    return (
      this.isDir() && (
        this.dirs() > 0 || this.files() > 0 ||
        this.links() > 0 || this.others() > 0
      )
    );
  }
  isRegular() { return this.typeBits() === ModeBits.Regular; }
  isSymlink() { return this.typeBits() === ModeBits.Symlink; }
  isGitlink() { return this.typeBits() === ModeBits.Gitlink; }
  mtime() { return moment.unix(this.stat().mtime); }
  size() { return this.stat().size; }
  symlink() { return this.stat().symlink; }
  gitlink() { return arrayToHex(this.stat().gitlink); }
  dirs() { return this.stat().dirs; }
  files() { return this.stat().files; }
  links() { return this.stat().links; }
  others() { return this.stat().others; }
}

function createFilesCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameFiles);
  return new Mongo.Collection(name, {
    transform: doc => new FsoFileC(doc),
  });
}

class FsoTreeErrorC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  message() { return this.d[KeyTreeErrorMessage]; }
}

function createTreeErrorsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameTreeErrors);
  return new Mongo.Collection(name, {
    transform: doc => new FsoTreeErrorC(doc),
  });
}

class FsoFileContentC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  repoPath() { return this.d[KeyRepoName]; }
  treePath() { return this.d[KeyTreePath]; }
  text() { return this.d[KeyContent]; }
}

function createContentCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameContent);
  return new Mongo.Collection(name, {
    transform: doc => new FsoFileContentC(doc),
  });
}

function createCollections({ namespace }) {
  // Client-only collections.
  return {
    files: createFilesCollection({ namespace }),
    treeErrors: createTreeErrorsCollection({ namespace }),
    content: createContentCollection({ namespace }),
  };
}

// The `subscribeX()` functions are bound to a fixed subscriber, usually
// `Meteor` or a testing mock.  If we wanted to support Blaze template
// `this.subscribe(...)`, we would add `subscribeXSubscriber()` functions.
function createSubscribeFuncs({ namespace, subscriber }) {
  return {
    subscribeTree(opts) {
      check(opts, { repoName: String });
      const pubName = makePubName(namespace, PubNameTree);
      return subscriber.subscribe(pubName, opts);
    },

    subscribeTreePathContent(opts) {
      check(opts, {
        repoName: String,
        treePath: String,
      });
      const pubName = makePubName(namespace, PubNameTreePathContent);
      return subscriber.subscribe(pubName, opts);
    },
  };
}

function createFsoTreeModuleClient({
  namespace, testAccess, subscriber,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(subscriber, Match.ObjectIncluding({ subscribe: Function }));

  const module = {
    ...createCollections({ namespace }),
    ...createSubscribeFuncs({ namespace, subscriber }),
  };
  return module;
}

export {
  createFsoTreeModuleClient,
};
