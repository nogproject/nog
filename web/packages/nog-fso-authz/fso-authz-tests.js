/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { testingDescribeStatements } from 'meteor/nog-access-2';

import { compileFsoPermissions } from 'meteor/nog-fso-authz';

function describeRules(name, { rules, expectations }) {
  testingDescribeStatements(
    { describe, it, expect },
    name, compileFsoPermissions(rules), expectations,
  );
}

function describeFsoAuthzTests() {
  describe('compileFsoPermissions()', function () {
    const action = 'fso/fake';
    const name = 'exreg';

    // `actionReal` is a real action that works with `AllowInsecureEverything`.
    const actionReal = 'fso/admin-registry';

    const ldapPrefix = 'ou_';
    const ldapPrefix1 = 'srv_';
    const ldapPrefix2 = 'ou_';
    const aliceUsername = 'alice';
    const aliceUserDoc = {
      username: aliceUsername,
      services: {
        gitexample: { ldapgroups: ['srv_microscope', 'ou_ag-alice'] },
      },
    };

    // Principals:
    const alice = 'username:alice';
    const agAlice = 'ldapgroup:ou_ag-alice';
    const bob = 'username:bob';
    const agBob = 'ldapgroup:ou_ag-bob';

    const effectAllow = { effect: 'allow' };
    const effectDeny = { effect: 'deny' };

    describeRules('AllowInsecureEverything', {
      rules: [
        {
          rule: 'AllowInsecureEverything',
          usernames: [aliceUsername],
        },
      ],
      expectations: [
        {
          name: 'allow matching user',
          principals: [alice],
          action: actionReal, opts: { name },
          ...effectAllow,
        },
        {
          name: 'deny other user',
          principals: [bob],
          action: actionReal, opts: { name },
          ...effectDeny,
        },
      ],
    });

    describeRules('AllowPrincipalsNames', {
      rules: [
        {
          rule: 'AllowPrincipalsNames',
          actions: [action],
          names: [name],
          principals: [alice, agAlice],
        },
      ],
      expectations: [
        {
          name: 'allow matching user',
          principals: [alice],
          action, opts: { name },
          ...effectAllow,
        },
        {
          name: 'allow matching group',
          principals: [agAlice],
          action, opts: { name },
          ...effectAllow,
        },
        {
          name: 'deny other user',
          principals: [bob],
          action, opts: { name },
          ...effectDeny,
        },
        {
          name: 'deny other group',
          principals: [agBob],
          action, opts: { name },
          ...effectDeny,
        },
      ],
    });

    describeRules('AllowPrincipalsPathPrefix', {
      rules: [
        {
          rule: 'AllowPrincipalsPathPrefix',
          actions: [action],
          pathPrefix: '/foo/',
          principals: [alice, agAlice],
        },
      ],
      expectations: [
        {
          name: 'allow matching user, matching path',
          principals: [alice],
          action, opts: { path: '/foo/bar' },
          ...effectAllow,
        },
        {
          name: 'allow matching group, matching path',
          principals: [agAlice],
          action, opts: { path: '/foo/bar' },
          ...effectAllow,
        },
        {
          name: 'deny other user, matching path',
          principals: [bob],
          action, opts: { path: '/foo/bar' },
          ...effectDeny,
        },
        {
          name: 'deny other group, matching path',
          principals: [agBob],
          action, opts: { path: '/foo/bar' },
          ...effectDeny,
        },
        {
          name: 'deny matching user, other path',
          principals: [alice],
          action, opts: { path: '/bar/foo' },
          ...effectDeny,
        },
        {
          name: 'deny matching group, other path',
          principals: [agAlice],
          action, opts: { path: '/bar/foo' },
          ...effectDeny,
        },
      ],
    });

    describeRules('AllowPrincipalsPathEqualOrPrefix', {
      rules: [
        {
          rule: 'AllowPrincipalsPathEqualOrPrefix',
          actions: [action],
          path: '/foo',
          principals: [alice, agAlice],
        },
      ],
      expectations: [
        {
          name: 'allow matching user, equal path',
          principals: [alice],
          action, opts: { path: '/foo' },
          ...effectAllow,
        },
        {
          name: 'allow matching group, equal path',
          principals: [agAlice],
          action, opts: { path: '/foo' },
          ...effectAllow,
        },
        {
          name: 'allow matching user, matching path',
          principals: [alice],
          action, opts: { path: '/foo/bar' },
          ...effectAllow,
        },
        {
          name: 'deny other user, equal path',
          principals: [bob],
          action, opts: { path: '/foo' },
          ...effectDeny,
        },
        {
          name: 'deny other group, equal path',
          principals: [agBob],
          action, opts: { path: '/foo' },
          ...effectDeny,
        },
        {
          name: 'deny other user, matching path',
          principals: [bob],
          action, opts: { path: '/foo/bar' },
          ...effectDeny,
        },
        {
          name: 'deny matching user, other path',
          principals: [alice],
          action, opts: { path: '/bar/foo' },
          ...effectDeny,
        },
      ],
    });

    describeRules('AllowPrincipalsPathPattern', {
      rules: [
        {
          rule: 'AllowPrincipalsPathPattern',
          actions: [action],
          pathPattern: '/foo/(bar|baz)',
          principals: [alice, agAlice],
        },
      ],
      expectations: [
        {
          name: 'allow matching user, matching path',
          principals: [alice],
          action, opts: { path: '/foo/bar' },
          ...effectAllow,
        },
        {
          name: 'allow matching group, matching path',
          principals: [agAlice],
          action, opts: { path: '/foo/baz' },
          ...effectAllow,
        },
        {
          name: 'deny other user, matching path',
          principals: [bob],
          action, opts: { path: '/foo/bar' },
          ...effectDeny,
        },
        {
          name: 'deny other group, matching path',
          principals: [agBob],
          action, opts: { path: '/foo/baz' },
          ...effectDeny,
        },
        {
          name: 'deny matching user, other path',
          principals: [alice],
          action, opts: { path: '/foo/barmore' },
          ...effectDeny,
        },
        {
          name: 'deny matching group, other path',
          principals: [agAlice],
          action, opts: { path: '/bar/foo' },
          ...effectDeny,
        },
      ],
    });

    describeRules('AllowLdapGroupFromPath', {
      rules: [
        {
          rule: 'AllowLdapGroupFromPath',
          pathPattern: '/orgfs/:group/(.*)?',
          ldapPrefix,
          actions: [action],
        },
      ],
      expectations: [
        {
          name: 'allow matching group, matching path',
          principals: [agAlice],
          action, opts: { path: '/orgfs/ag-alice' },
          ...effectAllow,
        },
        {
          name: 'allow matching group, matching sub-path',
          principals: [agAlice],
          action, opts: { path: '/orgfs/ag-alice/foo' },
          ...effectAllow,
        },
        {
          name: 'deny group path mismatch',
          principals: [agBob],
          action, opts: { path: '/orgfs/ag-alice' },
          ...effectDeny,
        },
      ],
    });

    describeRules('AllowLdapGroups2FromPath', {
      rules: [
        {
          rule: 'AllowLdapGroups2FromPath',
          pathPattern: '/orgfs/:group1/:group2/(.*)?',
          ldapPrefix1,
          ldapPrefix2,
          actions: [action],
        },
      ],
      expectations: [
        {
          name: 'allow matching group path',
          principals: [
            'ldapgroup:srv_microscope',
            'ldapgroup:ou_ag-alice',
          ],
          action, opts: {
            path: '/orgfs/microscope/ag-alice',
            user: aliceUserDoc,
          },
          ...effectAllow,
        },
        {
          name: 'deny mismatch group service path',
          principals: [
            'ldapgroup:srv_microscope',
            'ldapgroup:ou_ag-alice',
          ],
          action, opts: {
            path: '/orgfs/microscope2/ag-alice',
            user: aliceUserDoc,
          },
          ...effectDeny,
        },
        {
          name: 'deny mismatch group ou path',
          principals: [
            'ldapgroup:srv_microscope',
            'ldapgroup:ou_ag-alice',
          ],
          action, opts: {
            path: '/orgfs/microscope/ag-bob',
            user: aliceUserDoc,
          },
          ...effectDeny,
        },
      ],
    });
  });
}

export {
  describeFsoAuthzTests,
};
