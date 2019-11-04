/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { checkUserPluginScopes } from './scopes-legacy.js';

function describeScopesLegacyTests() {
  // See comment in `./authz.js`
  describe('nog-error legacy checkUserPluginScopes', function () {
    const fakeActionFoo = 'actionFoo';
    const fakeActionBar = 'actionBar';
    const fakeOptsFoo = { optFoo: 'foo' };
    const fakeOptsBar = { optBar: 'bar' };

    it('allows matching scope.', function () {
      const fakeUser = {
        scopes: [
          { action: fakeActionFoo, opts: fakeOptsFoo },
        ],
      };
      function fn() {
        checkUserPluginScopes(fakeUser, fakeActionFoo, fakeOptsFoo);
      }
      expect(fn).to.not.throw();
    });

    it('allows more than matching scope.', function () {
      const fakeUser = {
        scopes: [
          { action: fakeActionFoo, opts: fakeOptsFoo },
          { action: fakeActionBar, opts: fakeOptsBar },
        ],
      };
      function fn() {
        checkUserPluginScopes(fakeUser, fakeActionBar, fakeOptsBar);
      }
      expect(fn).to.not.throw();
    });

    it('denies scope opts mismatch.', function () {
      const fakeUser = {
        scopes: [
          { action: fakeActionFoo, opts: fakeOptsBar },
        ],
      };
      function fn() {
        checkUserPluginScopes(fakeUser, fakeActionFoo, fakeOptsFoo);
      }
      expect(fn).to.throw('ERR_ACCESS_DENY');
      expect(fn).to.throw('key opts mismatch');
    });

    it('denies insufficient scope.', function () {
      const fakeUser = {
        scopes: [
          { action: fakeActionBar, opts: fakeOptsBar },
        ],
      };
      function fn() {
        checkUserPluginScopes(fakeUser, fakeActionFoo, fakeOptsFoo);
      }
      expect(fn).to.throw('ERR_ACCESS_DENY');
      expect(fn).to.throw('Insufficient key scope');
    });
  });
}

export {
  describeScopesLegacyTests,
};
