import connect from 'connect';
import pathToRegexp from 'path-to-regexp';
import bodyParser from 'body-parser';
import { createError, nogthrow } from 'meteor/nog-error-2';
import { parse as urlparse } from 'url';
import * as _ from './underscore.js';

const optDebug = false;

const ERR_INTERNAL = {
  errorCode: 'ERR_INTERNAL',
  statusCode: 500,
  sanitized: null,
  reason: 'Internal server error',
};

const ERR_UNEXPECTED_EXCEPTION = {
  errorCode: 'ERR_UNEXPECTED_EXCEPTION',
  statusCode: 500,
  sanitized: null,
  reason: 'Internal server error',
};

const ERR_LIMIT = {
  errorCode: 'ERR_LIMIT',
  statusCode: 413,
  sanitized: 'full',
  reason: 'The request is larger than a limit.',
};

const ERR_PATH_NOT_FOUND = {
  errorCode: 'ERR_PATH_NOT_FOUND',
  statusCode: 404,
  sanitized: 'full',
  reason: 'Path not found.',
};

function baseUrl(req) {
  const { originalUrl, url } = req;
  if (!originalUrl.endsWith(url)) {
    nogthrow(ERR_INTERNAL, {
      reason: 'Original URL does not end with local URL.',
    });
  }
  let base = originalUrl.substr(0, originalUrl.length - url.length);
  if (base === '') {
    base = '/';
  }
  return base;
}

// `notFound()` is a middleware that unconditionally raises
// `ERR_PATH_NOT_FOUND`.
function notFound(req, res, next) {
  next(createError(ERR_PATH_NOT_FOUND));
}

function endOk(res, result) {
  const statusCode = (result && result.statusCode) || 200;
  const data = _.omit(result || {}, 'statusCode');
  res.writeHead(statusCode, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ statusCode, data }));
}

function endRedirect(res, result) {
  res.writeHead(result.statusCode, {
    'Content-Type': 'application/json',
    Location: result.location,
  });
  res.end(JSON.stringify(result));
}

// `endError()` is a middleware that handles all errors.
//
// It handles some known error types:
//
//  - Meteor and Nog errors use `errorType`.
//  - body-parser errors use `type`, see
//    <https://github.com/expressjs/body-parser#errors>.
//
function endError(err, req, res, next) { // eslint-disable-line no-unused-vars
  const body = {};
  if (err.errorType === 'Match.Error') {
    body.statusCode = 422;
    body.errorCode = 'ERR_MATCH';
    body.message = err.message;
    if (optDebug) {
      body.errorObject = err;
    }
  } else if (err.errorType === 'Meteor.Error') {
    body.errorCode = err.error;
    body.statusCode = err.statusCode || ERR_INTERNAL.statusCode;
    body.message = err.message;
    body.details = err.details;
    if (optDebug) {
      body.errorObject = err;
    }
  } else if (err.errorType === 'NogError.Error') {
    body.errorCode = err.sanitizedError.error;
    body.statusCode = err.statusCode || ERR_INTERNAL.statusCode;
    body.message = err.sanitizedError.message;
    body.details = err.sanitizedError.details;
    if (optDebug) {
      body.errorObject = err;
    }
  } else if (err.type === 'entity.too.large') {
    body.statusCode = ERR_LIMIT.statusCode;
    body.errorCode = ERR_LIMIT.errorCode;
    body.message = (
      `The body size ${err.length} `
      + `is larger than the limit ${err.limit}.`
    );
  } else {
    body.statusCode = ERR_UNEXPECTED_EXCEPTION.statusCode;
    body.errorCode = ERR_UNEXPECTED_EXCEPTION.errorCode;
    body.message = ERR_UNEXPECTED_EXCEPTION.reason;
    console.error(
      `[nog-rest-2] Unexpected JavaScript error: `
      + `${err.message}\n${err.stack}`,
    );
    if (optDebug) {
      body.message = `Unexpected JavaScript error: ${err.message}`;
      body.errorObject = { stack: err.stack };
    }
  }
  res.writeHead(body.statusCode, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
}

function createRestServer({
  limit = '10mb',
}) {
  const app = connect();

  // Use sub-app for auth middlewares, so that they run first even though they
  // will be attached later.
  const auth = connect();
  app.use(auth);

  app.use(bodyParser.json({ limit }));

  // Use a sub-app for actions, so that they run before the error handler even
  // though they will be attached later.
  const appActions = connect();
  app.use(appActions);

  // Treat unknown path as error.  It seems wrong to pass the request to
  // middlewares that run after this server, because this server may have
  // tainted the request by applying `auth` and `bodyParser`.
  app.use(notFound);

  // Handle all errors.
  app.use(endError);

  function useAction(route) {
    const rgxKeys = [];
    const rgx = pathToRegexp(route.path, rgxKeys);

    // `async` ensures a new Meteor fiber if needed.
    appActions.use(async (req, res, next) => {
      if (req.method !== route.method) {
        return next();
      }

      const parsed = urlparse(req.url, /* parseQueryString: */true);
      const m = parsed.pathname.match(rgx);
      if (!m) {
        return next();
      }
      const params = {};
      for (let i = 1; i < m.length; i += 1) {
        const k = rgxKeys[i - 1].name;
        const v = m[i];
        params[k] = v;
      }
      req.params = params;
      req.query = parsed.query;

      // `baseUrl` like Express; see
      // <https://expressjs.com/en/api.html#req.baseUrl>.
      req.baseUrl = baseUrl(req);

      try {
        const result = route.action(req);
        const sc = (result && result.statusCode) || 200;
        if (sc >= 300 && sc < 400) {
          return endRedirect(res, result);
        }
        return endOk(res, result);
      } catch (err) {
        return next(err);
      }
    });
  }

  function useActions(routes) {
    routes.forEach(useAction);
  }

  const server = {
    app,
    auth,
    useActions,
  };
  return server;
}

export {
  createRestServer,
};
