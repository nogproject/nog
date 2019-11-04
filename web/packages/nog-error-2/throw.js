import { Meteor } from 'meteor/meteor';
import { check, Match } from 'meteor/check';
import { Random } from 'meteor/random';
import {
  clone,
  isFunction,
  isObject,
  omit,
  pick,
} from './underscore.js';

const NogErrorType = Meteor.makeErrorType(
  // Named `NogError.Error` for backward compatibility with `nog-error`.
  'NogError.Error',
  function ctor(opts) {
    Object.assign(this, pick(opts,
      'errorCode', 'reason', 'details', 'statusCode', 'time', 'token',
      'history', 'context',
    ));
    this.message = `${this.reason} [${this.errorCode}]`;
    const s = opts.sanitized;
    this.sanitizedError = new Meteor.Error(s.errorCode, s.reason, s.details);
  },
);

const sanitizedDefaults = {
  errorCode: 'NOGERR',
  reason: 'Unspecified error.',
  details: '',
};

function sanitize(errdoc, spec, ctx) {
  if (!spec.sanitized) {
    return { ...sanitizedDefaults };
  }

  if (spec.sanitized === 'full') {
    const s = pick(errdoc, 'errorCode', 'reason', 'details');
    if (!s.details) {
      s.details = '';
    }
    return s;
  }

  if (isObject(spec.sanitized)) {
    const s = { ...spec.sanitized };
    if (!s.errorCode) {
      s.errorCode = sanitizedDefaults.errorCode;
    }
    if (isFunction(s.reason)) {
      s.reason = s.reason(ctx);
    }
    if (!s.reason) {
      s.reason = sanitizedDefaults.reason;
    }
    if (isFunction(s.details)) {
      s.details = s.details(ctx);
    }
    if (!s.details) {
      s.details = sanitizedDefaults.details;
    }
    return s;
  }

  console.error(
    `[nog-error] Invalid value \`spec.sanitized: ${spec.sanitized}\`; `
    + 'using defaults.',
  );
  return { ...sanitizedDefaults };
}

function cloneOwn(obj) {
  return Object.assign({}, obj);
}

function errForHistory(err) {
  return {
    errorCode: err.errorCode || err.code || err.error,
    statusCode: err.statusCode,
    reason: err.reason || err.message,
    details: err.details || null,
    sanitized: clone(err.sanitized || null),
    time: err.time || null,
    token: err.token || null,
    context: err.context || omit(cloneOwn(err),
      'errorCode', 'code', 'error', 'statusCode', 'reason', 'message',
      'details', 'sanitized', 'time', 'token', 'errorType', 'history',
      'sanitizedError',
    ),
  };
}

function causeTail(cause) {
  let reason = '';
  if (cause.reason) {
    reason = ` Cause: ${cause.reason}`;
  } else if (cause.message) {
    reason = ` Cause: ${cause.message}`;
  }

  let details = '';
  if (cause.details) {
    details = ` Cause: ${cause.details}`;
  }

  let history;
  if (Match.test(cause.history, [Object])) {
    ({ history } = cause);
  } else {
    history = [errForHistory(cause)];
  }

  let sanitizedReason = '';
  let sanitizedDetails = '';
  const serr = cause.sanitizedError;
  if (serr) {
    sanitizedReason = ` Cause: ${serr.reason}`;
    sanitizedDetails = ` Cause: ${serr.details}`;
  } else if (cause.errorType === 'Meteor.Error') {
    sanitizedReason = ` Cause: ${cause.reason}`;
    sanitizedDetails = ` Cause: ${cause.details}`;
  }

  return {
    reason, details, history, sanitizedReason, sanitizedDetails,
  };
}

function createErrorModule({ platform }) {
  function fmtToken(msg, tok, time) {
    let m = '[';
    m += platform.where; // client or server.
    if (time) {
      m += ' ';
      m += time.toISOString();
    }
    m += ' ';
    m += tok;
    m += ']';
    if (msg && msg.length) {
      m += ' ';
      m += msg;
    } else {
      m += '.';
    }
    return m;
  }

  const logError = Meteor.bindEnvironment((errdoc) => {
    const { errorLog } = platform;
    if (!errorLog) {
      return;
    }
    // Pass callback to force async call, ignoring errors.
    errorLog.insert(errdoc, () => {});
  });

  function createErrorWithSpec(spec, ctx = {}) {
    const { errorCode, contextPattern } = spec;
    if (contextPattern) {
      try {
        check(ctx, Match.ObjectIncluding(contextPattern));
      } catch (err) {
        console.error(
          `[nog-error-2] The context for ${errorCode} does not have `
          + `the expected structure: ${err.message}.`,
        );
      }
    }

    const { statusCode } = spec;
    const errdoc = {
      errorCode,
      statusCode,
      context: omit(ctx, 'cause', 'reason', 'details'),
    };

    if (ctx.reason) {
      errdoc.reason = String(ctx.reason);
    } else if (isFunction(spec.reason)) {
      errdoc.reason = spec.reason(ctx);
    } else {
      errdoc.reason = String(spec.reason);
    }

    if (ctx.details) {
      errdoc.details = String(ctx.details);
    } else if (isFunction(spec.details)) {
      const d = spec.details(ctx);
      if (isObject(d)) {
        errdoc.details = d.details;
        Object.assign(errdoc.context, omit(d, 'details'));
      } else {
        errdoc.details = d;
      }
    } else {
      errdoc.details = spec.details;
    }

    errdoc.sanitized = sanitize(errdoc, spec, ctx);

    errdoc.time = new Date();
    errdoc.token = Random.id(6).toLowerCase();
    errdoc.history = [errForHistory(errdoc)];

    errdoc.details = fmtToken(errdoc.details, errdoc.token, errdoc.time);
    errdoc.sanitized.details = fmtToken(
      errdoc.sanitized.details, errdoc.token, errdoc.time,
    );

    if (ctx.cause) {
      const t = causeTail(ctx.cause);
      errdoc.reason += t.reason;
      errdoc.details += t.details;
      if (spec.sanitized === 'full') {
        errdoc.sanitized.reason += t.reason;
        errdoc.sanitized.details += t.details;
      } else {
        errdoc.sanitized.reason += t.sanitizedReason;
        errdoc.sanitized.details += t.sanitizedDetails;
      }
      errdoc.history = errdoc.history.concat(t.history);
    }

    logError(errdoc);

    return new NogErrorType(errdoc);
  }

  function createError(spec, ctx) {
    if (!(
      isObject(spec) && (ctx === undefined || isObject(ctx))
    )) {
      throw new Error(
        'Invalid call to `createError()`: '
        + 'nog-error legacy interface not supported.',
      );
    }
    return createErrorWithSpec(spec, ctx);
  }

  function nogthrow(...args) {
    throw createError(...args);
  }

  const module = {
    createError,
    nogthrow,
  };
  return module;
}

export {
  createErrorModule,
};
