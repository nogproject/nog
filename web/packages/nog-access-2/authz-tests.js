/* eslint-env mocha */
/* eslint-disable func-names */
import chai from 'chai';
import sinon from 'sinon';
import sinonChai from 'sinon-chai';
chai.use(sinonChai);
const { expect } = chai;

import { Mongo } from 'meteor/mongo';
import { Random } from 'meteor/random';

import { createAccessModuleServer } from 'meteor/nog-access-2';

// Directly import implementation to test without a full server module.
import { createAuthorizer, principalPluginsDefault } from './authz.js';

function describeAuthzTests() {
  describe('authorizer.checkAccessPrincipals()', function () {
    let authz = null;
    beforeEach(function () {
      authz = createAuthorizer({});
    });

    const specs = [
      {
        statements: [
          { principal: 'user', action: 'do', effect: 'allow' },
        ],
        expectations: [
          {
            name: 'denies access by default.',
            principals: ['foo'],
            action: 'do',
            opts: null,
            effect: 'deny',
          },
          {
            name: 'allows access by a matching statement.',
            principals: ['user'],
            action: 'do',
            opts: null,
            effect: 'allow',
          },
        ],
      },

      {
        statements: [
          { principal: 'user', action: 'do', effect: 'allow' },
          { principal: 'foo', action: 'do', effect: 'deny' },
        ],
        expectations: [
          {
            name: 'deny statement overrides allow statement.',
            principals: ['user', 'foo'],
            action: 'do',
            opts: null,
            effect: 'deny',
          },
        ],
      },

      {
        statements: [
          { principal: 'user', action: 'do', effect: opts => opts.res },
        ],
        expectations: [
          {
            name: 'effect can be specified as a function (allow).',
            principals: ['user'],
            action: 'do',
            opts: { res: 'allow' },
            effect: 'allow',
          },
          {
            name: 'effect can be specified as a function (deny).',
            principals: ['user'],
            action: 'do',
            opts: { res: 'deny' },
            effect: 'deny',
          },
        ],
      },

      {
        statements: [
          {
            principal: 'user',
            action: 'do',
            effect: () => ({ effect: 'deny', reason: 'testingReason' }),
          },
        ],
        expectations: [
          {
            name: 'effect function can return object to describe reason.',
            principals: ['user'],
            action: 'do',
            opts: { res: 'deny' },
            effect: 'deny',
            messages: ['testingReason'],
          },
        ],
      },

      {
        statements: [
          {
            principal: /^user/,
            action: 'do',
            effect: (opts) => {
              if (opts.principal === 'user') {
                return { effect: 'deny', reason: 'exact match' };
              }
              return { effect: 'deny', reason: 'prefix match' };
            },
          },
        ],
        expectations: [
          {
            name: 'principal can be a RegExp.',
            principals: ['user'],
            action: 'do',
            opts: {},
            effect: 'deny',
            messages: ['exact match'],
          },
          {
            name: 'principal can be a RegExp.',
            principals: ['user2'],
            action: 'do',
            opts: {},
            effect: 'deny',
            messages: ['prefix match'],
          },
        ],
      },
    ];

    specs.forEach(({ statements, expectations }) => {
      expectations.forEach(({
        name, principals, action, opts, effect, messages,
      }) => {
        it(name, function () {
          authz.addStatements(statements);
          function fn() {
            authz.checkAccessPrincipals(principals, action, opts);
          }
          switch (effect) {
            case 'deny':
              expect(fn).to.throw('denied');
              if (messages) {
                messages.forEach((msg) => {
                  expect(fn).to.throw(msg);
                });
              }
              break;
            case 'allow':
              expect(fn).to.not.throw();
              break;
            default:
              throw new Error('logic error');
          }
        });
      });
    });
  });

  describe('with fake users', function () {
    const fakeUserAlice = {
      _id: Random.id(),
      username: 'alice',
      roles: ['users'],
      services: {
        gitfoo: { ldapgroups: ['foounix'] },
        gitbar: { ldapgroups: ['barunix'] },
      },
    };
    const users = new Mongo.Collection(null);
    users.insert(fakeUserAlice);

    const fakeActionFoo = 'actionFoo';
    const fakeOpts = { optFoo: 'bar' };

    describe('authorizer plugins mechanism', function () {
      it('calls checkUserPlugins.', function () {
        const checkUserPlugins = [
          sinon.fake(),
          sinon.fake(),
        ];
        const authz = createAuthorizer({
          users,
          checkUserPlugins,
          principalPlugins: principalPluginsDefault,
        });
        authz.testAccess(fakeUserAlice, fakeActionFoo, fakeOpts);
        expect(checkUserPlugins[0]).to.have.been.calledOnceWith(
          fakeUserAlice, fakeActionFoo, fakeOpts,
        );
        expect(checkUserPlugins[1]).to.have.been.calledOnceWith(
          fakeUserAlice, fakeActionFoo, fakeOpts,
        );
      });

      it('calls principalPlugins to map user to principals.', function () {
        const authz = createAuthorizer({
          users,
          principalPlugins: [
          // Full args: (principals, user) => ... .
            (principals) => { principals.push('role:foo'); },
            (principals) => { principals.push('role:bar'); },
          ],
        });
        const fakeCheckAccessPrincipals = sinon.fake();
        sinon.replace(
          authz, 'checkAccessPrincipals', fakeCheckAccessPrincipals,
        );
        authz.checkAccess(fakeUserAlice, fakeActionFoo);
        expect(fakeCheckAccessPrincipals).to.have.been.calledWith(
          ['role:foo', 'role:bar'], fakeActionFoo, sinon.match.object,
        );
        sinon.restore();
      });
    });

    describe('authorizer.testAccess()', function () {
      const fakeUserAlicePrincipals = [
        'role:users',
        'username:alice',
        `userid:${fakeUserAlice._id}`,
        'ldapgroup:foounix',
        'ldapgroup:barunix',
      ];

      const authz = createAuthorizer({
        users,
        principalPlugins: principalPluginsDefault,
      });

      let fakeCheckAccessPrincipals = null;
      beforeEach(function () {
        fakeCheckAccessPrincipals = sinon.fake();
        sinon.replace(
          authz, 'checkAccessPrincipals', fakeCheckAccessPrincipals,
        );
      });
      afterEach(function () {
        sinon.restore();
      });

      it('maps null euid to principal `anonymous`.', function () {
        const euid = null;
        authz.checkAccess(euid, fakeActionFoo);
        expect(fakeCheckAccessPrincipals).to.have.been.calledWith(
          ['anonymous'], fakeActionFoo, {},
        );
      });

      it('maps unknown user to principal `anonymous`.', function () {
        const euid = 'unknownId';
        authz.checkAccess(euid, fakeActionFoo);
        expect(fakeCheckAccessPrincipals).to.have.been.calledWith(
          ['anonymous'], fakeActionFoo, {},
        );
      });

      it('maps user to principals.', function () {
        authz.checkAccess(fakeUserAlice, fakeActionFoo);
        expect(fakeCheckAccessPrincipals).to.have.been.calledWith(
          fakeUserAlicePrincipals, fakeActionFoo, { user: fakeUserAlice },
        );
      });

      it('maps uid to principals.', function () {
        authz.checkAccess(fakeUserAlice._id, fakeActionFoo);
        expect(fakeCheckAccessPrincipals).to.have.been.calledWith(
          fakeUserAlicePrincipals, fakeActionFoo, { user: fakeUserAlice },
        );
      });

      it('passes through checkAccess() opts.', function () {
        const euid = null;
        authz.checkAccess(euid, fakeActionFoo, fakeOpts);
        expect(fakeCheckAccessPrincipals).to.have.been.calledWith(
          ['anonymous'], fakeActionFoo, fakeOpts,
        );
      });

      it('extends checkAccess() opts with user.', function () {
        authz.checkAccess(fakeUserAlice, fakeActionFoo, fakeOpts);
        expect(fakeCheckAccessPrincipals).to.have.been.calledWith(
          sinon.match.any, fakeActionFoo, { ...fakeOpts, user: fakeUserAlice },
        );
      });
    });

    describe('module', function () {
      it('calls checkUserPlugins.', function () {
        const checkUserPlugins = [
          sinon.fake(),
          sinon.fake(),
        ];
        const module = createAccessModuleServer({
          namespace: { meth: Random.id() },
          users,
          checkUserPlugins,
        });
        module.testAccess(fakeUserAlice, fakeActionFoo);
        expect(checkUserPlugins[0]).to.have.been.calledOnceWith(
          fakeUserAlice,
        );
        expect(checkUserPlugins[1]).to.have.been.calledOnceWith(
          fakeUserAlice,
        );
      });

      it('calls principalPlugins.', function () {
        const principalPlugins = [
          sinon.fake(),
          sinon.fake(),
        ];
        const module = createAccessModuleServer({
          namespace: { meth: Random.id() },
          users,
          principalPlugins,
        });
        const granted = module.testAccess(fakeUserAlice, fakeActionFoo);
        expect(granted).to.equal(false);
        expect(principalPlugins[0]).to.have.been.calledOnceWith(
          sinon.match.array, fakeUserAlice,
        );
        expect(principalPlugins[1]).to.have.been.calledOnceWith(
          sinon.match.array, fakeUserAlice,
        );
      });
    });
  });
}

export {
  describeAuthzTests,
};
