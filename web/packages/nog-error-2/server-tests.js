/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { Mongo } from 'meteor/mongo';

import { nogthrow, createErrorModule } from 'meteor/nog-error-2';
import { ERR_FAKE } from './testlib.js';

import './common-tests.js';

describe('nog-error-2 server', function () {
  it('throws "server" in error details', function () {
    function fn() {
      nogthrow(ERR_FAKE);
    }
    expect(fn).to.throw().with.property('details').that.matches(/server /);
  });

  it('supports late binding of logging collection', function () {
    const platform = {
      where: 'server',
      errorLog: null,
    };
    const NogError = createErrorModule({ platform });
    const errorLog = new Mongo.Collection(null);
    platform.errorLog = errorLog;
    try {
      NogError.nogthrow(ERR_FAKE);
    } catch (err) {
      const err2 = errorLog.findOne();
      expect(err.details).to.equal(err2.details);
    }
  });
});
