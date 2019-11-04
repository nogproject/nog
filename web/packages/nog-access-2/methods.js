import { Meteor } from 'meteor/meteor';
import { check, Match } from 'meteor/check';

// `defMethodCalls()` registers Meteor methods and binds them to the
// server-side method implementation on `module`.  `module` must be `null` on
// the client.  The common code applies loose checks to catch obvious errors.
// The server methods on `module` must apply the strict checks.
function defMethodCalls(module, { namespace }) {
  if (Meteor.isServer) {
    check(module, Match.ObjectIncluding({
      testAccess: Function,
    }));
  } else {
    check(module, null);
  }

  // For each call, register a Meteor method and return a wrapper function that
  // calls the method.
  function def(calls) {
    const wrapped = {};
    for (const [name, fn] of Object.entries(calls)) {
      const qualname = `${namespace.meth}.${name}`;
      Meteor.methods({ [qualname]: fn });
      wrapped[name] = (...args) => Meteor.call(qualname, ...args);
    }
    return wrapped;
  }

  return def({
    callTestAccess(action, opts) {
      check(action, String);
      check(opts, Object);
      if (!module) {
        return null;
      }
      return module.testAccess(Meteor.user(), action, opts);
    },
  });
}

export {
  defMethodCalls,
};
