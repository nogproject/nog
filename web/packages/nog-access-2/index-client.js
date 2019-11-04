import { EJSON } from 'meteor/ejson';
import { check } from 'meteor/check';
import { ReactiveDict } from 'meteor/reactive-dict';
import { defMethodCalls } from './methods.js';
import {
  isFunction,
  isObject,
  isUndefined,
} from './underscore.js';

function createAccessModuleClient({
  namespace, userId,
}) {
  check(namespace, { meth: String });
  check(userId, Function);

  const { callTestAccess } = defMethodCalls(null, { namespace });

  const cache = new ReactiveDict();

  function testAccess(action, optsOrCallback, callbackOrUndefined) {
    const opts = isObject(optsOrCallback) ? optsOrCallback : {};
    let callback = null;
    if (isFunction(optsOrCallback)) {
      callback = optsOrCallback;
    } else if (isFunction(callbackOrUndefined)) {
      callback = callbackOrUndefined;
    }

    const uid = userId();
    const key = EJSON.stringify(
      { uid, action, opts },
      { canonical: true },
    );
    const val = cache.get(key);

    // Use the cached value; but also call the server to check whether it has
    // changed.
    if (val != null) {
      callTestAccess(action, opts, (err, res) => {
        if (res != null) {
          cache.set(key, res);
        }
      });
      if (callback) {
        callback(null, val);
      }
      return val;
    }

    if (isUndefined(val) || callback) {
      cache.set(key, null);
      callTestAccess(action, opts, (err, res) => {
        if (res != null) {
          cache.set(key, res);
        }
        if (callback) {
          callback(err, res);
        }
      });
    }

    return null;
  }

  const module = {
    testAccess,
  };
  return module;
}

export {
  createAccessModuleClient,
};
