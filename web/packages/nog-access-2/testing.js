/* eslint-disable func-names */

import { check, Match } from 'meteor/check';

import { matchStatement } from './statements.js';
import { createAuthorizer } from './authz.js';

const matchExpectationEffect = Match.Where((x) => {
  check(x, String);
  return x === 'allow' || x === 'deny';
});

const matchExpectation = Match.Where((x) => {
  check(x, {
    name: String,
    principals: [String],
    action: String,
    opts: Match.Optional(Object),
    effect: matchExpectationEffect,
  });
  return true;
});

// `testingDescribeStatements()` is a helper function to test statements in
// mocha tests.  Example:
//
// ```
// describe('statements', function () {
//   testingDescribeStatements(
//     { describe, it, expect },
//     'StatementsIsRoleX', StatementsIsRoleX,
//     [
//       {
//         name: 'isGuest(guests) is true.',
//         principals: ['guests'],
//         action: 'isGuest',
//         opts: { path: '/' },
//         effect: 'allow',
//       },
//     ],
//   );
// });
// ```
function testingDescribeStatements(
  { describe, it, expect },
  statementsName, statements, expectations,
) {
  check(describe, Function);
  check(it, Function);
  check(expect, Function);
  check(statementsName, String);
  check(statements, [matchStatement]);
  check(expectations, [matchExpectation]);

  describe(statementsName, function () {
    const authz = createAuthorizer({});
    authz.addStatements(statements);
    expectations.forEach(({
      name, principals, action, opts, effect,
    }) => {
      it(name, function () {
        function fn() {
          authz.checkAccessPrincipals(principals, action, opts);
        }
        switch (effect) {
          case 'deny':
            expect(fn).to.throw('denied');
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
}

export {
  testingDescribeStatements,
};
