import { check, Match } from 'meteor/check';


const matchPositiveNumber = Match.Where(x => {
  check(x, Number);
  if (x <= 0) {
    throw new Match.Error(`Expected positive number; got ${x}.`);
  }
  return true;
});


const matchNonNegativeNumber = Match.Where(x => {
  check(x, Number);
  if (x < 0) {
    throw new Match.Error(`Expected non-negative number; got ${x}.`);
  }
  return true;
});


export {
  matchPositiveNumber,
  matchNonNegativeNumber,
};
