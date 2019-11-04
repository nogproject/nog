import * as _ from './underscore.js';

function createComputer({
  name,
}) {
  function sum(a, b) {
    return a + b;
  }

  function nameSum(a, b) {
    return `${name}: ${sum(a, b)}`;
  }

  // Instead of simply `return { sum, nameSum }`, use an underscore function to
  // illustrate how to import it as a npm peer dependency.
  const comp = {};
  _.extend(comp, {
    sum,
    nameSum,
  });
  return comp;
}

export {
  createComputer,
};
