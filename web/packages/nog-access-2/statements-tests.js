/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import {
  StatementsIsRoleX,
  testingDescribeStatements,
} from 'meteor/nog-access-2';

function describeStatementsTests() {
  describe('statements', function () {
    testingDescribeStatements(
      { describe, it, expect },
      'StatementsIsRoleX', StatementsIsRoleX,
      [
        {
          name: 'isGuest(guests) is true.',
          principals: ['guests'],
          action: 'isGuest',
          effect: 'allow',
        },
        {
          name: 'isUser(guests) is false.',
          principals: ['guests'],
          action: 'isUser',
          effect: 'deny',
        },
        {
          name: 'isAdmin(guests) is false.',
          principals: ['guests'],
          action: 'isAdmin',
          effect: 'deny',
        },

        {
          name: 'isGuest(role:users) is false.',
          principals: ['role:users'],
          action: 'isGuest',
          effect: 'deny',
        },
        {
          name: 'isUser(role:users) is true.',
          principals: ['role:users'],
          action: 'isUser',
          effect: 'allow',
        },
        {
          name: 'isAdmin(role:users) is false.',
          principals: ['role:users'],
          action: 'isAdmin',
          effect: 'deny',
        },

        {
          name: 'isGuest(role:admins) is false.',
          principals: ['role:admins'],
          action: 'isGuest',
          effect: 'deny',
        },
        {
          name: 'isUser(role:admins) is false.',
          principals: ['role:admins'],
          action: 'isUser',
          effect: 'deny',
        },
        {
          name: 'isAdmin(role:admins) is true.',
          principals: ['role:admins'],
          action: 'isAdmin',
          effect: 'allow',
        },
      ],
    );
  });
}

export {
  describeStatementsTests,
};
