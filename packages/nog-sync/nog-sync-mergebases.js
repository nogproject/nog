// `mergeBases()` implements Git's paint down algorithm to detect multiple
// merge bases; see `paint_down_to_common` in `commit.c`.

import PriorityQueue from 'priorityqueuejs';

const OURS = (1 << 0);
const THEIRS = (1 << 1);
const STALE = (1 << 2);
const RESULT = (1 << 3);


function mergeBases({ ourSha, theirSha, store }) {
  const flagStore = {};

  function getFlags(c) {
    return flagStore[c._id] || 0;
  }

  function addFlags(c, flags) {
    flagStore[c._id] = getFlags(c) | flags;
  }

  // q returns the highest element first, like a reversed sort.
  const q = new PriorityQueue((a, b) => a.commitDate - b.commitDate);

  function haveNonstale() {
    // eslint-disable-next-line no-underscore-dangle
    for (const c of q._elements) {
      if (!(getFlags(c) & STALE)) {
        return true;
      }
    }
    return false;
  }

  let c;

  c = store.getCommitOrNull(ourSha);
  if (c == null) {
    return [];
  }
  addFlags(c, OURS);
  q.enq(c);

  c = store.getCommitOrNull(theirSha);
  if (c == null) {
    return [];
  }
  addFlags(c, THEIRS);
  q.enq(c);

  const results = [];
  while (haveNonstale()) {
    c = q.deq();
    let flags = getFlags(c) & (OURS | THEIRS | STALE);

    if (flags === (OURS | THEIRS)) {
      if (!(getFlags(c) & RESULT)) {
        addFlags(c, RESULT);
        results.push(c);
      }
      // Mark parents of found merge base as stale.
      flags |= STALE;
    }

    for (const psha of c.parents) {
      const p = store.getCommitOrNull(psha);

      // Ignore missing parents.
      if (p == null) {
        continue;
      }

      // Skip if already painted with expected flags.
      if ((getFlags(p) & flags) === flags) {
        continue;
      }

      addFlags(p, flags);
      q.enq(p);
    }
  }

  // Sort in descending commit order.
  results.sort((a, b) => b.commitDate - a.commitDate);
  return results.map((r) => r._id);
}

export { mergeBases };
