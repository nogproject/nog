// @flow

/*

The following timings should give an idea of the expected cache performance.
The timings were recorded with a different cache implementation.  But the
general pattern should hold: high hit ratio, order of magnitude lower running
time.

 - Without cache, 200 repos: merge: 2 - 5 seconds
 - With cache, 200 repos, hit ratio 98%: merge 0.1 - 0.9 seconds
 - With cache, 1000 repos, hit ratio 99.4%: same time as with 200 repos.

*/

type Entry = { _id: string };

type createEntryCache$opts = {
  maxNElements: number,
  maxAge_s: number,
  name: string,
};

function createEntryCache(
  {
    maxNElements = (32 * 1024),
    maxAge_s = (10 * 60),  // eslint-disable-line camelcase
    name = 'unknown',
  } : createEntryCache$opts
) {
  return {
    maxNElements,
    maxAge_s,
    name,
    entries: {},
    nElements: 0,
    nHits: 0,
    nMisses: 0,
    ctime_ms: Date.now(),

    clear() {
      this.entries = {};
      this.trees = {};
      this.objects = {};
      this.nElements = 0;
      this.nHits = 0;
      this.nMisses = 0;
      this.ctime_ms = Date.now();
    },

    maybeClear() {
      if (this.age_s() > this.maxAge_s || this.nElements > this.maxNElements) {
        const nAccesses = this.nHits + this.nMisses;
        console.log(
          `[caching] '${this.name}' cache before clear:`,
          `nAccesses: ${nAccesses},`,
          `nElements: ${this.nElements},`,
          `hitRatio: ${this.hitRatio().toFixed(4)}`,
        );
        this.clear();
      }
    },

    age_s() {
      return (Date.now() - this.ctime_ms) / 1000;
    },

    hitRatio() {
      if (this.nHits === 0) {
        return 0;
      }
      return this.nHits / (this.nHits + this.nMisses);
    },

    add(entry: Entry) {
      this.maybeClear();
      if (this.entries[entry._id] != null) {
        return;
      }
      this.nElements += 1;
      this.entries[entry._id] = entry;
    },

    get(sha: string) {
      this.maybeClear();
      const e = this.entries[sha];
      if (e == null) {
        this.nMisses += 1;
      } else {
        this.nHits += 1;
      }
      return e;
    },
  };
}

function createEntryNullCache() {
  return {
    add() {},
    get() {},
  };
}

export { createEntryCache, createEntryNullCache };
