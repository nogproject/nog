// The db check code should not use code from the other packages.  It should
// implement the checks as independent code paths for redundancy, even if it
// means to duplicate code.

import { Meteor } from 'meteor/meteor';
import { Mongo } from 'meteor/mongo';
import { _ } from 'meteor/underscore';
import { EJSON } from 'meteor/ejson';
import { check, Match } from 'meteor/check';
import crypto from 'crypto';

const AA_OPS_DBCK = 'nog-ops/dbck';
const NULL_SHA1 = '0000000000000000000000000000000000000000';

const maxUploadDurationH = 10;

function logerr(err) {
  console.error(`[dbck] ${err}`);
}

// `yieldFiber()` gives other tasks a chance to run.  It should be called often
// to keep the dbck in the background.  Calling it only every 5ths sha loop
// iteration already caused a noticeably less responsive reload in other
// browser windows.
function yieldFiber() {
  Meteor._sleepForMs(0);
}

function createProgressLogger({ what }) {
  return {
    what,
    idx: 0,
    count() {
      this.idx += 1;
      if (this.idx % 10000 === 0) {
        console.log(`[dbck] ${this.idx} ${this.what} have been scanned...`);
      }
    },
    end() {
      console.log(`[dbck] Completed scan of ${this.idx} ${this.what}.`);
    },
  };
}

// Use fresh `Collection` instances to fetch untransformed content.  Disable
// mutation methods to avoid duplicate global Meteor method definitions, since
// `Collection` has been already instantiated in package `nog-content` if it is
// loaded.
//
// I(spr) briefly checked `meteor/packages/mongo/collection.js`.  It seems safe
// to instantiate a `Collection` with the same name multiple times.

function open(name) {
  return new Mongo.Collection(name, { defineMutationMethods: false });
}

function openRawContentCollections() {
  return {
    repos: open('repos'),
    commits: open('commits'),
    trees: open('trees'),
    objects: open('objects'),
    blobs: open('blobs'),
  };
}

function openRawSharingCollections() {
  return {
    shares: open('shares'),
    users: open('users'),
    repos: open('repos'),
  };
}

function openRawCatalogCollections() {
  return {
    repos: open('repos'),
    fsoRepos: open('fso.repos'),
    catalogs: open('catalogs'),
  };
}

// `verifyBlobStatus()` checks basic blob invariants:
//
//  - Uploads should have completed and be verified within reasonable time.
//  - The daemons have not reported blob-specific errors.
function verifyBlobStatus() {
  const errors = [];
  const { blobs } = openRawContentCollections();

  const nErr = blobs.find({ errors: { $exists: true } }).count();
  if (nErr > 0) {
    const err = `${nErr} blobs with errors.`;
    logerr(err);
    errors.push(err);
  }

  const cutoff = new Date();
  cutoff.setHours(cutoff.getHours() - maxUploadDurationH);

  const nStaleUploads = blobs.find({
    mtime: { $lt: cutoff },
    status: 'uploading',
  }).count();
  if (nStaleUploads > 0) {
    const err = (
      `${nStaleUploads} uploading blobs ` +
      `without activity for more than ${maxUploadDurationH}h.`
    );
    logerr(err);
    errors.push(err);
  }

  const nUnverified = blobs.find({
    mtime: { $lt: cutoff },
    status: 'available', $or: [
      { verified: { $exists: false } },
      { 'locs.verified': { $exists: false } },
    ],
  }).count();
  if (nUnverified > 0) {
    const err = (
      `${nUnverified} unverified blobs ` +
      `without activity for more than ${maxUploadDurationH}h.`
    );
    logerr(err);
    errors.push(err);
  }

  return { ok: (errors.length === 0), errors };
}

function sha1Hex(txt) {
  return crypto.createHash('sha1').update(txt, 'utf8').digest('hex');
}

function decodeMeta(enc) {
  const more = enc.more || [];
  const dec = _.omit(enc, 'more');
  for (const f of more) {
    dec[f.key] = f.val;
  }
  return dec;
}

function stripNonEssential(c) {
  return _.omit(c, '_id', '_idversion', 'errata');
}

function contentId(c) {
  return sha1Hex(EJSON.stringify(c, { canonical: true }));
}

function contentShaIsOk(rawDoc) {
  const id = rawDoc._id;
  const doc = stripNonEssential(rawDoc);
  doc.meta = decodeMeta(doc.meta);
  return contentId(doc) === id;
}

// `verifyContentShas()` verifies the id, computing the content SHA1 using an
// independent code path.
function verifyContentShas(coll) {
  const errors = [];
  const progress = createProgressLogger({
    what: `content shas of '${coll._name}'`,
  });
  coll.find({}).forEach((doc) => {
    yieldFiber();
    if (!contentShaIsOk(doc)) {
      const err = `Content sha mismatch for '${coll._name}/${doc._id}'.`;
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();
  return { ok: (errors.length === 0), errors };
}

function hasOneOfErrata(content, eras) {
  const { errata } = content;
  if (!errata) {
    return false;
  }
  for (const era of errata) {
    if (_.contains(eras, era.code)) {
      return true;
    }
  }
  return false;
}

// `verifyConnectivity()` checks that all entry dependencies are available,
// ignoring known errata.
function verifyConnectivity() {
  const errors = [];
  const {
    repos, commits, trees, objects, blobs,
  } = openRawContentCollections();

  const { log } = console;
  let progress;

  function haveCommit(sha) {
    return commits.findOne({ _id: sha }, { fields: { _id: 1 } }) != null;
  }

  function haveTree(sha) {
    return trees.findOne({ _id: sha }, { fields: { _id: 1 } }) != null;
  }

  function haveObject(sha) {
    return objects.findOne({ _id: sha }, { fields: { _id: 1 } }) != null;
  }

  function haveBlob(sha) {
    return blobs.findOne({ _id: sha }, { fields: { _id: 1 } }) != null;
  }

  function checkRepo(repo) {
    const { refs } = repo;
    for (const [ref, commit] of _.pairs(refs)) {
      if (commit == null) {
        continue; // eslint-disable-line no-continue
      }
      if (commit === NULL_SHA1) {
        return (
          `Invalid commit '${commit}' for ref '${ref}' ` +
          `of repo '${repo.owner}/${repo.name}'.`
        );
      }
      if (haveCommit(commit)) {
        continue; // eslint-disable-line no-continue
      }
      return (
        `Missing commit '${commit}' for ref '${ref}' ` +
        `of repo '${repo.owner}/${repo.name}'.`
      );
    }
    return null;
  }

  function checkCommit(commit) {
    const { tree, parents } = commit;
    for (const p of parents) {
      if (!haveCommit(p)) {
        return `Missing parent '${p}' of commit '${commit._id}'.`;
      }
    }
    if (!haveTree(tree)) {
      return `Missing tree '${tree}' of commit '${commit._id}'.`;
    }
    return null;
  }

  function checkTree(tree) {
    const { entries } = tree;
    for (const ent of entries) {
      if (ent.type === 'tree') {
        if (!haveTree(ent.sha1)) {
          return `Missing child tree '${ent.sha1}' of tree '${tree._id}'.`;
        }
      } else if (ent.type === 'object') {
        if (!haveObject(ent.sha1)) {
          return `Missing child object '${ent.sha1}' of tree '${tree._id}'.`;
        }
      } else {
        return `Invalid entry type '${ent.type}' in tree '${tree._id}'.`;
      }
    }
    return null;
  }

  // `checkObject()` ignores the following errata:
  //
  // - ERA201507a: Blobs with SHA1 mismatch due to Rusha bug were removed in
  //   2015.
  // - ERA201609a: Blobs with SHA1 mismatch due to Rusha 2 GiB bug may have
  //   been removed.
  // - ERA201609b: Blobs with SHA1 mismatch due to size 0 upload were removed.
  function checkObject(obj) {
    const { blob } = obj;
    const knownErrata = ['ERA201507a', 'ERA201609a', 'ERA201609b'];
    if (hasOneOfErrata(obj, knownErrata)) {
      return null;
    }
    if (blob == null) {
      return null;
    }
    if (blob === NULL_SHA1) {
      return null;
    }
    if (haveBlob(blob)) {
      return null;
    }
    return `Missing blob '${blob}' for object '${obj._id}'.`;
  }

  log('[dbck] Verifying repo connectivity.');
  progress = createProgressLogger({ what: 'repos' });
  repos.find(
    {},
    { fields: { owner: 1, name: 1, refs: 1 } },
  ).forEach((repo) => {
    yieldFiber();
    const err = checkRepo(repo);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  log('[dbck] Verifying commit connectivity.');
  progress = createProgressLogger({ what: 'commits' });
  commits.find({}, { fields: { tree: 1, parents: 1 } }).forEach((commit) => {
    yieldFiber();
    const err = checkCommit(commit);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  log('[dbck] Verifying tree connectivity.');
  progress = createProgressLogger({ what: 'trees' });
  trees.find({}, { fields: { entries: 1 } }).forEach((tree) => {
    yieldFiber();
    const err = checkTree(tree);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  log('[dbck] Verifying object connectivity.');
  progress = createProgressLogger({ what: 'objects' });
  objects.find({}, { fields: { blob: 1, errata: 1 } }).forEach((obj) => {
    yieldFiber();
    const err = checkObject(obj);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  return { ok: (errors.length === 0), errors };
}

// `verifySharing()` checks invariants of the sharing data structures, which
// are distributed over several collections.  The data structures are explained
// in `nog-app/meteor/nog-sharing.coffee`.
function verifySharing() {
  const errors = [];
  const { shares, users, repos } = openRawSharingCollections();

  let progress;

  function contains(coll, sel) {
    return coll.findOne(sel, { fields: { _id: 1 } }) != null;
  }

  function checkShare(share) {
    const {
      fromId, circleId, toId, toName,
    } = share;

    if (!contains(users, { _id: fromId, 'sharing.circles._id': circleId })) {
      return (
        `Failed to find circle '${circleId}' in sharing user '${fromId}' ` +
        `for share '${share._id}'.`
      );
    }

    if (!contains(
      users, { _id: toId, 'sharing.inCircles': { circleId, fromId } },
    )) {
      return (
        `Failed to find circle '${circleId}' of sharing user '${fromId}' ` +
        `in circle member user '${toId}' for share '${share._id}'.`
      );
    }

    if (!contains(users, { _id: toId, username: toName })) {
      return (
        `Circle member name '${toName}' does not match user '${toId}' ` +
        `for share '${share._id}'.`
      );
    }

    return null;
  }

  function checkRepo(repo) {
    const { ownerId, sharing } = repo;
    if (sharing == null || sharing.circles == null) {
      return null;
    }
    for (const cid of sharing.circles) {
      if (!contains(users, { _id: ownerId, 'sharing.circles._id': cid })) {
        return (
          `Failed to find circle '${cid}' in sharing user '${ownerId}' ` +
          `for repo '${repo._id}'.`
        );
      }
    }
    return null;
  }

  function checkUser(user) {
    const { sharing } = user;
    if (sharing == null || sharing.inCircles == null) {
      return null;
    }
    for (const { circleId, fromId } of sharing.inCircles) {
      if (!contains(users, { _id: fromId, 'sharing.circles._id': circleId })) {
        return (
          `Failed to find circle '${circleId}' ` +
          `in sharing user '${fromId}' ` +
          `for circle member user '${user._id}'.`
        );
      }
      if (!contains(shares, { circleId, fromId, toId: user._id })) {
        return (
          `Failed to find share for circle '${circleId}' ` +
          `from sharing user '${fromId}' for circle member user '${user._id}'.`
        );
      }
    }
    return null;
  }

  progress = createProgressLogger({ what: 'shares' });
  shares.find({}).forEach((share) => {
    yieldFiber();
    const err = checkShare(share);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  progress = createProgressLogger({ what: 'repos' });
  repos.find({}, { fields: { sharing: 1, ownerId: 1 } }).forEach((repo) => {
    yieldFiber();
    const err = checkRepo(repo);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  progress = createProgressLogger({ what: 'users' });
  users.find({}, { fields: { sharing: 1 } }).forEach((user) => {
    yieldFiber();
    const err = checkUser(user);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  return { ok: (errors.length === 0), errors };
}

// `verifyCatalog()` checks that there is no stale catalog state.
function verifyCatalog() {
  const errors = [];
  const { repos, fsoRepos, catalogs } = openRawCatalogCollections();

  function contains(coll, sel) {
    return coll.findOne(sel, { fields: { _id: 1 } }) != null;
  }

  function checkCatalog(catalog) {
    const {
      _id: catalogId, owner, name, fsoPath,
    } = catalog;
    const sel = { _id: catalogId };
    if (owner) {
      if (!contains(repos, sel)) {
        return (
          `Stale catalog '${catalogId}' '${owner}/${name}' ` +
          `without corresponding repo.`
        );
      }
    } else if (fsoPath) {
      if (!contains(fsoRepos, sel)) {
        return (
          `Stale catalog '${catalogId}' '${fsoPath}' ` +
          `without corresponding FSO repo.`
        );
      }
    } else {
      return `Catalog '${catalogId}' of unknown type.`;
    }
    return null;
  }

  function checkVolume(volumeInfo) {
    const { name } = volumeInfo;
    const m = name.match(/^[^.]*[.]vol_([^_]*)_\d*$/);
    if (!m) {
      return `Invalid catalog volume name '${name}'.`;
    }
    const id = m[1];
    const sel = { _id: id };
    if (!contains(repos, sel) && !contains(fsoRepos, sel)) {
      return (
        `Stale catalog volume collection '${name}' ` +
        `without corresponding repo.`
      );
    }
    return null;
  }

  let progress;

  progress = createProgressLogger({ what: 'catalogs' });
  catalogs.find({}).forEach((share) => {
    yieldFiber();
    const err = checkCatalog(share);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  progress = createProgressLogger({ what: 'volumes' });
  const cursor = catalogs._driver.mongo.db.listCollections({
    name: { $regex: /^catalogs[.]/ },
  });
  cursor.toArraySync = Meteor.wrapAsync(cursor.toArray);
  cursor.toArraySync().forEach((v) => {
    const err = checkVolume(v);
    if (err != null) {
      logerr(err);
      errors.push(err);
    }
    progress.count();
  });
  progress.end();

  return { ok: (errors.length === 0), errors };
}

function dbckSudo({
  checks = ['blobstatus', 'sha', 'connectivity', 'sharing', 'catalog'],
}) {
  const { log } = console;

  function fmtRes(res) {
    return res.ok ? 'ok' : `${res.errors.length} errors`;
  }

  const checkFns = {
    blobstatus: verifyBlobStatus,
    catalog: verifyCatalog,
    connectivity: verifyConnectivity,
    sharing: verifySharing,
  };

  let errors = [];
  log(`[dbck] Begin checks: ${checks.join(', ')}.`);

  for (const chk of checks) {
    if (chk === 'sha') {
      const { commits, trees, objects } = openRawContentCollections();
      for (const coll of [commits, trees, objects]) {
        log(`[dbck] Begin verify content shas of '${coll._name}'.`);
        const res = verifyContentShas(coll);
        log(
          `[dbck] End verify content shas of '${coll._name}': ${fmtRes(res)}.`,
        );
        errors = errors.concat(res.errors);
      }
    } else if (checkFns[chk]) {
      log(`[dbck] Begin verify ${chk}.`);
      const res = checkFns[chk]();
      log(`[dbck] End verify ${chk}: ${fmtRes(res)}.`);
      errors = errors.concat(res.errors);
    } else {
      const err = `Unknown check \`${chk}\`.`;
      logerr(err);
      errors.push(err);
    }
  }

  const result = { ok: (errors.length === 0), errors };
  log(`[dbck] End checks: ${fmtRes(result)}.`);
  return result;
}

function createDbChecker({ checkAccess }) {
  return {
    dbck(euid, opts) {
      check(opts, Match.Optional({ checks: [String] }));
      checkAccess(euid, AA_OPS_DBCK);
      return dbckSudo(opts || {});
    },
  };
}

export { createDbChecker };
