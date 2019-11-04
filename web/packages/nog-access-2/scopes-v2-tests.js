/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { checkUserPluginScopesV2 } from './scopes-v2.js';

function describeScopesV2Tests() {
  describe('checkUserPluginScopesV2', function () {
    const fakeActionFoo = 'action/foo';
    const fakeActionBar = 'action/bar';
    const fakeNameFoo = 'foo';
    const fakeNameBar = 'bar';
    const fakePathFoo = '/data/foo';
    const fakePathBar = '/data/bar';
    const fakeOptsNameFoo = { name: fakeNameFoo };
    const fakeOptsNameBar = { name: fakeNameBar };
    const fakeOptsPathFoo = { path: fakePathFoo };
    const fakeOptsPathBar = { path: fakePathBar };

    const fakeActionOther = 'otheraction/other';
    const fakeNameOther = 'other';
    const fakePathOther = '/otherdata/other';
    const fakeOptsNameOther = { name: fakeNameOther };
    const fakeOptsPathOther = { path: fakePathOther };

    const Effect = {
      allow: 'allow',
      deny: 'deny',
    };

    const expectations = [
      {
        name: 'allows name in scope.',
        scopesV2: [
          { actions: [fakeActionFoo], names: [fakeNameFoo] },
          { actions: [fakeActionFoo], names: [fakeNameBar] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsNameBar,
        effect: Effect.allow,
      },
      {
        name: 'allows path in scope.',
        scopesV2: [
          { actions: [fakeActionFoo], paths: [fakePathFoo] },
          { actions: [fakeActionFoo], paths: [fakePathBar] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsPathBar,
        effect: Effect.allow,
      },

      {
        name: 'allows action wildcard in scope.',
        scopesV2: [
          { actions: ['*'], names: [fakeNameFoo] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsNameFoo,
        effect: Effect.allow,
      },
      {
        name: 'allows name wildcard in scope.',
        scopesV2: [
          { actions: [fakeActionFoo], names: ['*'] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsNameFoo,
        effect: Effect.allow,
      },
      {
        name: 'allows path wildcard scope.',
        scopesV2: [
          { actions: [fakeActionFoo], paths: ['*'] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsPathFoo,
        effect: Effect.allow,
      },

      {
        name: 'allows action prefix wildcard in scope.',
        scopesV2: [
          { actions: ['action/*'], names: ['fo*'] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsNameFoo,
        effect: Effect.allow,
      },
      {
        name: 'allows name prefix wildcard in scope.',
        scopesV2: [
          { actions: [fakeActionFoo], names: ['fo*'] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsNameFoo,
        effect: Effect.allow,
      },
      {
        name: 'allows path prefix scope.',
        scopesV2: [
          { actions: [fakeActionFoo], paths: ['/data/*'] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsPathFoo,
        effect: Effect.allow,
      },

      {
        name: 'denies scope missing action.',
        scopesV2: [
          { actions: [fakeActionFoo, fakeActionBar], names: [fakeNameFoo] },
        ],
        action: fakeActionOther,
        opts: fakeOptsNameFoo,
        effect: Effect.deny,
      },
      {
        name: 'denies scope missing name.',
        scopesV2: [
          { actions: [fakeActionFoo], names: [fakeNameFoo, fakeNameBar] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsNameOther,
        effect: Effect.deny,
      },
      {
        name: 'denies scope missing path.',
        scopesV2: [
          { actions: [fakeActionFoo], paths: [fakePathFoo, fakePathBar] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsPathOther,
        effect: Effect.deny,
      },

      {
        name: 'denies mismatching scope action wildcard.',
        scopesV2: [
          { actions: ['action/*'], names: [fakeNameFoo] },
        ],
        action: fakeActionOther,
        opts: fakeOptsNameFoo,
        effect: Effect.deny,
      },
      {
        name: 'denies mismatching scope name wildcard.',
        scopesV2: [
          { actions: [fakeActionFoo], names: ['fo*', 'ba*'] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsNameOther,
        effect: Effect.deny,
      },
      {
        name: 'denies mismatching scope path wildcard.',
        scopesV2: [
          { actions: [fakeActionFoo], paths: ['/data/*'] },
        ],
        action: fakeActionFoo,
        opts: fakeOptsPathOther,
        effect: Effect.deny,
      },
    ];

    expectations.forEach(({
      name, scopesV2, action, opts, effect,
    }) => {
      it(name, function () {
        const fakeUser = { scopesV2 };
        function fn() {
          checkUserPluginScopesV2(fakeUser, action, opts);
        }
        switch (effect) {
          case Effect.allow:
            expect(fn).to.not.throw();
            return;
          case Effect.deny:
            expect(fn).to.throw('ERR_ACCESS_DENY');
            return;
          default:
            throw new Error('logic error');
        }
      });
    });
  });
}

export {
  describeScopesV2Tests,
};
