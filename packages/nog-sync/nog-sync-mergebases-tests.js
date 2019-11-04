/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */

import {
  describe, it,
} from 'meteor/practicalmeteor:mocha';

import { expect } from 'meteor/practicalmeteor:chai';
import { _ } from 'meteor/underscore';

import { mergeBases } from './nog-sync-mergebases.js';


function makeGraph(nodes) {
  const graph = {};
  let fakeDate = 1;
  for (let n of nodes) {
    n = _.clone(n);
    n.commitDate = fakeDate;
    fakeDate += 1;
    graph[n._id] = n;
  }
  return graph;
}

describe('nog-sync', function () {
  describe('mergeBases()', function () {
    let graph = null;
    const store = {
      getCommitOrNull(sha) {
        const node = graph[sha];
        if (node != null) {
          return _.clone(node);
        }
        return null;
      },
    };

    it('returns empty without common ancestor', function () {
      graph = makeGraph([
        { _id: 'a', parents: [] },
        { _id: 'b', parents: [] },
      ]);
      const mbs = mergeBases({ ourSha: 'a', theirSha: 'b', store });
      expect(mbs).to.be.empty;
    });

    it('fast-forward', function () {
      graph = makeGraph([
        { _id: 'a', parents: [] },
        { _id: 'b', parents: ['a'] },
        { _id: 'b2', parents: ['b'] },
      ]);
      let mbs;
      mbs = mergeBases({ ourSha: 'a', theirSha: 'b2', store });
      expect(mbs).to.deep.eql(['a']);
      mbs = mergeBases({ ourSha: 'b2', theirSha: 'a', store });
      expect(mbs).to.deep.eql(['a']);
    });

    it('finds common ancestor', function () {
      graph = makeGraph([
        { _id: 'base', parents: [] },
        { _id: 'a', parents: ['base'] },
        { _id: 'a2', parents: ['a'] },
        { _id: 'b', parents: ['base'] },
        { _id: 'b2', parents: ['b'] },
      ]);
      const mbs = mergeBases({ ourSha: 'a2', theirSha: 'b2', store });
      expect(mbs).to.deep.eql(['base']);
    });

    it('stops traversal in commit-date order', function () {
      // It won't enter the trap, since it uses commit-date order.
      graph = makeGraph([
        { _id: 'trap', parents: ['trap'] },
        { _id: 'base', parents: ['trap'] },
        { _id: 'a', parents: ['base'] },
        { _id: 'b', parents: ['base'] },
      ]);
      const mbs = mergeBases({ ourSha: 'a', theirSha: 'b', store });
      expect(mbs).to.deep.eql(['base']);
    });

    it('finds latest common ancestor', function () {
      //
      //  base -- a -- base2 ---- a2
      //    \            \
      //     b --------- merge -- b2
      //
      graph = makeGraph([
        { _id: 'base', parents: [] },
        { _id: 'a', parents: ['base'] },
        { _id: 'b', parents: ['base'] },
        { _id: 'base2', parents: ['a'] },
        { _id: 'merge', parents: ['base2', 'b'] },
        { _id: 'a2', parents: ['base2'] },
        { _id: 'b2', parents: ['merge'] },
      ]);
      const mbs = mergeBases({ ourSha: 'a2', theirSha: 'b2', store });
      expect(mbs).to.deep.eql(['base2']);
    });

    it('ignores missing commits', function () {
      graph = makeGraph([
        { _id: 'a', parents: [] },
        { _id: 'a2', parents: ['a'] },
        { _id: 'b2', parents: ['b'] },
      ]);

      let mbs;

      mbs = mergeBases({ ourSha: 'a2', theirSha: 'c', store });
      expect(mbs).to.be.empty;

      mbs = mergeBases({ ourSha: 'a2', theirSha: 'b2', store });
      expect(mbs).to.be.empty;

      mbs = mergeBases({ ourSha: 'c', theirSha: 'a2', store });
      expect(mbs).to.be.empty;

      mbs = mergeBases({ ourSha: 'b2', theirSha: 'a2', store });
      expect(mbs).to.be.empty;
    });

    it('finds multiple ancestors in decending commit date order', function () {
      //
      //  base----a--base2--merge2--a2
      //    \            \ /
      //     \            \
      //      \          / \
      //       b----base3---merge3---b2
      //
      graph = makeGraph([
        { _id: 'base', parents: [] },
        { _id: 'a', parents: ['base'] },
        { _id: 'b', parents: ['base'] },
        { _id: 'base2', parents: ['a'] },
        { _id: 'base3', parents: ['b'] },
        { _id: 'merge2', parents: ['base2', 'base3'] },
        { _id: 'merge3', parents: ['base3', 'base2'] },
        { _id: 'a2', parents: ['merge2'] },
        { _id: 'b2', parents: ['merge3'] },
      ]);
      const mbs = mergeBases({ ourSha: 'a2', theirSha: 'b2', store });
      expect(mbs).to.deep.eql(['base3', 'base2']);
    });
  });
});
