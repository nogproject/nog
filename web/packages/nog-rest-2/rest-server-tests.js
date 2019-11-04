/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import http from 'http';
import connect from 'connect';
import { HTTP } from 'meteor/http';
import { createError } from 'meteor/nog-error-2';

import { createRestServer } from 'meteor/nog-rest-2';

const ERR_FAKE_UNAUTHORIZED = {
  errorCode: 'ERR_FAKE_UNAUTHORIZED',
  statusCode: 401,
  sanitized: 'full',
  reason: 'Unauthorized',
};

function describeRestServerTests() {
  describe('rest server', function () {
    let fakeWebAppConnectHandlers = null;
    let api = null;
    let server = null;
    let url = '';
    let urlApiFoo = '';

    beforeEach(async function () {
      fakeWebAppConnectHandlers = connect();
      api = createRestServer({});
      fakeWebAppConnectHandlers.use('/api', api.app);
      fakeWebAppConnectHandlers.use('/api2', api.app);
      server = http.createServer(fakeWebAppConnectHandlers);
      await new Promise((resolve, reject) => {
        try {
          server.listen(0, 'localhost', resolve);
        } catch (err) {
          reject(err);
        }
      });
      url = `http://localhost:${server.address().port}`;
      urlApiFoo = `${url}/api/foo`;
    });

    afterEach(function () {
      server.close();
    });

    const fooBar = { foo: 'bar' };
    const getFooAction = {
      method: 'GET',
      path: '/foo',
      action: () => fooBar,
    };

    it('handles unknown path.', function () {
      function fn() {
        HTTP.get(urlApiFoo);
      }
      expect(fn).to.throw('ERR_PATH_NOT_FOUND');
    });

    it('calls action.', function () {
      api.useActions([getFooAction]);
      const res = HTTP.get(urlApiFoo);
      expect(res.statusCode).to.equal(200);
      expect(res.data.data).to.eql(fooBar);
    });

    it('parses JSON body.', function () {
      let body = null;
      api.useActions([{
        method: 'POST',
        path: '/foo',
        action(req) {
          ({ body } = req);
          return {};
        },
      }]);
      HTTP.post(urlApiFoo, { data: fooBar });
      expect(body).to.eql(fooBar);
    });

    it('parses path params', function () {
      let params = null;
      api.useActions([{
        method: 'GET',
        path: '/foo/:foo',
        action(req) {
          ({ params } = req);
          return {};
        },
      }]);
      HTTP.get(`${urlApiFoo}/bar`);
      expect(params).to.eql(fooBar);
    });

    it('parses query.', function () {
      let query = null;
      api.useActions([{
        method: 'GET',
        path: '/foo',
        action(req) {
          ({ query } = req);
          return {};
        },
      }]);
      HTTP.get(`${urlApiFoo}?foo=bar`);
      expect(query).to.eql(fooBar);
    });

    it('sets baseUrl.', function () {
      let baseUrl = null;
      api.useActions([{
        method: 'GET',
        path: '/foo/:foo',
        action(req) {
          ({ baseUrl } = req);
          return {};
        },
      }]);
      HTTP.get(`${url}/api/foo/bar?baz=1`);
      expect(baseUrl).to.equal('/api');
      HTTP.get(`${url}/api2/foo/bar?baz=1`);
      expect(baseUrl).to.equal('/api2');
    });

    it('handles redirect.', function () {
      const newLocation = '/api3';
      api.useActions([{
        method: 'GET',
        path: '/foo',
        action() {
          return {
            statusCode: 301,
            location: newLocation,
          };
        },
      }]);
      const res = HTTP.get(urlApiFoo, { followRedirects: false });
      expect(res.statusCode).to.equal(301);
      expect(res.headers.location).to.equal(newLocation);
    });

    it('calls auth middleware.', function () {
      api.useActions([getFooAction]);
      let calledAuth = false;
      api.auth.use((req, res, next) => {
        calledAuth = true;
        next();
      });
      HTTP.get(urlApiFoo);
      expect(calledAuth).to.equal(true);
    });

    it('returns auth error.', function () {
      api.useActions([getFooAction]);
      api.auth.use((req, res, next) => {
        next(createError(ERR_FAKE_UNAUTHORIZED));
      });
      function fn() {
        HTTP.get(urlApiFoo);
      }
      expect(fn).to.throw(ERR_FAKE_UNAUTHORIZED.errorCode);
      try {
        fn();
      } catch (err) {
        const { errorCode, statusCode } = err.response.data;
        expect(errorCode).to.equal(ERR_FAKE_UNAUTHORIZED.errorCode);
        expect(statusCode).to.equal(ERR_FAKE_UNAUTHORIZED.statusCode);
      }
    });
  });
}

export {
  describeRestServerTests,
};
