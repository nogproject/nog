import crypto from 'crypto';
import { Meteor } from 'meteor/meteor';
import { Mongo } from 'meteor/mongo';
import { check, Match } from 'meteor/check';
import {
  ERR_FSO,
  nogthrow,
} from './errors.js';
import {
  CollNameRepoTars,
  CollNameTarttHeads,
  KeyAuthor,
  KeyCommitter,
  KeyId,
  KeyPath,
  KeyRepoId,
  KeyTarType,
  KeyTarttCommit,
  KeyTime,
  PubNameTartt,
  TarType,
} from './tartt.js';
import {
  TAR_FULL,
  TAR_PATCH,
} from './proto.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';

const AA_FSO_READ_REPO = 'fso/read-repo';

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

function nameHashId(name) {
  const s = crypto.createHash('sha1').update(name, 'utf8').digest('base64');
  // Shorten and replace confusing characters.
  return s.substr(0, 20).replace(/[=+/]/g, 'x');
}

class TarttHead {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
}

function createTarttHeadsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameTarttHeads);
  const tarttHeads = new Mongo.Collection(name, {
    transform: doc => new TarttHead(doc),
  });
  // No index: find searches only for primary key.
  return tarttHeads;
}

class RepoTar {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
}

function createRepoTarsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameRepoTars);
  const repoTars = new Mongo.Collection(name, {
    transform: doc => new RepoTar(doc),
  });
  repoTars.rawCollection().createIndex({ [KeyRepoId]: 1 });
  return repoTars;
}

function createCollections({ namespace }) {
  return {
    tarttHeads: createTarttHeadsCollection({ namespace }),
    repoTars: createRepoTarsCollection({ namespace }),
  };
}

// `parsePbTarInfo()` converts a protobuf `TarInfo` into a doc for MongoDB.
function parsePbTarInfo(repoId, pbT) {
  const tarType = {
    [TAR_FULL]: TarType.Full,
    [TAR_PATCH]: TarType.Patch,
  }[pbT.tarType];
  if (!tarType) {
    nogthrow(ERR_FSO, { reason: 'Invalid protobuf `TarInfo.TarType`.' });
  }

  return {
    id: nameHashId(`${repoId}:${pbT.path}`),
    path: pbT.path,
    tarType,
    time: new Date(Number.parseInt(pbT.time, 10) * 1000),
  };
}

function parsePbWhoDate(wd) {
  return {
    name: wd.name,
    email: wd.email,
    date: new Date(wd.date),
  };
}

// See `./tartt.js` for design overview.
function publishTarttFunc({
  tarttHeads, repoTars, openRepo,
}) {
  return function publishTartt(opts) {
    check(opts, {
      path: String,
    });
    const { path } = opts;

    // Rely on access check in `openRepo()`.
    const euid = this.userId ? Meteor.users.findOne(this.userId) : null;
    let repo;
    try {
      repo = openRepo(euid, {
        actions: [AA_FSO_READ_REPO],
        path,
      });
    } catch (err) {
      logerr('Failed to publish tartt.', 'path', path, 'err', err);
      this.ready();
      return null;
    }
    const { repoId } = repo;

    function poll() {
      const head = repo.tarttHead();
      if (tarttHeads.findOne({
        [KeyId]: repoId,
        [KeyTarttCommit]: head.commit,
      })) {
        return;
      }
      head.author = parsePbWhoDate(head.author);
      head.committer = parsePbWhoDate(head.committer);

      repo.listTars({ commit: head.commit }).tars.forEach((t) => {
        const info = parsePbTarInfo(repoId, t);
        repoTars.upsert(info.id, {
          [KeyId]: info.id,
          [KeyRepoId]: repoId,
          [KeyTarttCommit]: head.commit,
          [KeyPath]: info.path,
          [KeyTarType]: info.tarType,
          [KeyTime]: info.time,
        });
      });

      repoTars.remove({
        [KeyRepoId]: repoId,
        [KeyTarttCommit]: { $ne: head.commit },
      });

      tarttHeads.upsert(repoId, {
        [KeyId]: repoId,
        [KeyTarttCommit]: head.commit,
        [KeyAuthor]: head.author,
        [KeyCommitter]: head.committer,
      });
    }

    // Background polling while the publication is active:
    //
    // `tickInterval` is the period from the end of one poll until the next
    // poll.
    //
    // `tick` is the active timeout.  It is `null` if there is no active
    // timeout.
    //
    // `isStopped` ensures that polling stops even if `onStop()` happens while
    // `poll()` has not yet returned.
    //
    const tickInterval = 30 * 1000;
    let tick = null;
    let isStopped = false;

    const nextPoll = () => {
      try {
        poll();
      } catch (err) {
        // XXX Maybe set a tartt repo error that propagates to the client,
        // similar to `setRepoError()` in `./fso-pub-repo-poll-g2n.js`.
        logerr(
          'Failed to poll tartt.',
          'repoPath', path,
          'err', err,
        );
      }
      if (isStopped) {
        return;
      }
      tick = Meteor.setTimeout(nextPoll, tickInterval);
    };

    this.onStop(() => {
      if (tick) {
        Meteor.clearTimeout(tick);
      }
      isStopped = true;
    });

    Meteor.defer(nextPoll);

    const headFields = {
      [KeyId]: 1,
      [KeyAuthor]: 1,
      [KeyCommitter]: 1,
    };
    const tarFields = {
      [KeyId]: 1,
      [KeyRepoId]: 1,
      [KeyTarType]: 1,
      [KeyTime]: 1,
    };
    return [
      tarttHeads.find({ [KeyId]: repoId }, { fields: headFields }),
      repoTars.find({ [KeyRepoId]: repoId }, { fields: tarFields }),
    ];
  };
}

function registerPublications({
  publisher, namespace, tarttHeads, repoTars, openRepo,
}) {
  function defPub(name, fn) {
    publisher.publish(makePubName(namespace, name), fn);
  }
  defPub(PubNameTartt, publishTarttFunc({
    tarttHeads, repoTars, openRepo,
  }));
}

function createTarttModuleServer({
  namespace, testAccess, checkAccess, publisher, openRepo,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(checkAccess, Function);
  check(publisher, Match.ObjectIncluding({ publish: Function }));
  check(openRepo, Function);

  const { tarttHeads, repoTars } = createCollections({ namespace });

  registerPublications({
    publisher, namespace, tarttHeads, repoTars, openRepo,
  });

  return {
    tarttHeads, repoTars,
  };
}

export {
  createTarttModuleServer,
};
