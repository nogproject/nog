import { check, Match } from 'meteor/check';

const matchEffectString = Match.Where((x) => {
  check(x, String);
  return (x === 'allow' || x === 'deny');
});

const matchStatement = Match.Where((x) => {
  check(x, {
    principal: Match.OneOf(String, RegExp),
    action: String,
    effect: Match.OneOf(matchEffectString, Function),
  });
  return true;
});

// `StatementsIsRoleX` allow pseudo-actions `isUser`, `isAdmin`, and `isGuest`.
// `StatementsIsRoleX` should be avoided in favor of statements that allow
// specific actions.
const StatementsIsRoleX = [
  {
    principal: 'role:users',
    action: 'isUser',
    effect: 'allow',
  },
  {
    principal: 'role:admins',
    action: 'isAdmin',
    effect: 'allow',
  },
  {
    principal: 'guests',
    action: 'isGuest',
    effect: 'allow',
  },
];

export {
  matchStatement,
  StatementsIsRoleX,
};
