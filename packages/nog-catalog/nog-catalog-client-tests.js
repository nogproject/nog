/* global document */
/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */
/* eslint-disable max-len */

import { expect } from 'chai';
import sinon from 'sinon';

import { Blaze } from 'meteor/blaze';
import { Template } from 'meteor/templating';
import { $ } from 'meteor/jquery';
import './nog-catalog-ui.html';


function withDiv(callback) {
  const el = document.createElement('div');
  document.body.appendChild(el);
  try {
    callback(el);
  } finally {
    document.body.removeChild(el);
  }
}

function findElement(el, sel) {
  return $(el).find(sel);
}

function createFakeGlobals() {
  const called = {
    getParam: {
      ownerName: 0,
      repoName: 0,
    },
    subscribeCatalogArgs: null,
    subscribeCatalogHitCountArgs: null,
    subscribeCatalogVolumeArgs: null,
  };

  const fakeSubReady = {
    ready() {
      return true;
    },
  };

  const data = {
    router: {
      getParam(name) {
        called.getParam[name] += 1;
        return 'foo';
      },
      current() { return { queryParams: {} }; },
    },

    nogCatalog: {
      testAccess() {
        return true;
      },

      volumeRegistry: {
        getCollection() {
          return [];
        },
      },
      subscribeCatalog(...args) {
        called.subscribeCatalogArgs = args;
        return fakeSubReady;
      },
      subscribeCatalogHitCount(...args) {
        called.subscribeCatalogHitCountArgs = args;
        return fakeSubReady;
      },
      subscribeCatalogVolume(...args) {
        called.subscribeCatalogVolumeArgs = args;
        return fakeSubReady;
      },

      catalogs: {
        findOne() {
          return {
            active: {
              volumes: [
                { name: 'fakeVolume0' },
              ],
              metaKeys: ['fake_keywords'],
            },
          };
        },
      },

      callUpdateCatalog() {},
    },
  };

  const ctxCatalogTools = {
    ownerName: 'foo',
    repoName: 'foo',
    onUpdateCatalog() {},
    logMessages() {
      return {
        msgs: ['foo', 'bar', 'baz'],
        diffs: {
          added: [{ foo: 'bar' }],
          removed: [{ foo: 'bar' }],
          recount: [{ foo: 'bar' }],
        },
      };
    },
    testAccess() {
      return true;
    },
    isUpdating() {
      return false;
    },
  };

  return { data, called, ctxCatalogTools };
}


describe('nog-catalog', function () {
  describe('Template nogCatalogDiscoverPresenter', function () {
    it('subscriptions', function (done) {
      const { data, called } = createFakeGlobals();
      withDiv((el) => {
        Blaze.renderWithData(Template.nogCatalogDiscoverPresenter, data, el);
        expect(called.getParam.ownerName).to.be.at.least(1);
        expect(called.getParam.repoName).to.be.at.least(1);
        expect(called.subscribeCatalogArgs).to.exist;
        expect(called.subscribeCatalogHitCountArgs).to.exist;
        expect(called.subscribeCatalogVolumeArgs).to.exist;
        done();
      });
    });
    it('Renders update button for owner', function (done) {
      const { data } = createFakeGlobals();
      withDiv((el) => {
        Blaze.renderWithData(Template.nogCatalogDiscoverPresenter, data, el);
        const btn = findElement(el, '.js-catalog-update');
        expect(btn.length).to.eql(1);
        done();
      });
    });
    it('Renders NO update button for other users', function (done) {
      const { data } = createFakeGlobals();
      data.nogCatalog.testAccess = function () {
        return false;
      };
      withDiv((el) => {
        Blaze.renderWithData(Template.nogCatalogDiscoverPresenter, data, el);
        const btn = findElement(el, '.js-catalog-update');
        expect(btn.length).to.eql(0);
        done();
      });
    });
    it('Clicking update button triggers update method', function (done) {
      const { data } = createFakeGlobals();
      withDiv((el) => {
        Blaze.renderWithData(Template.nogCatalogDiscoverPresenter, data, el);
        const btn = findElement(el, '.js-catalog-update');
        expect(btn.length).to.eql(1);
        const save = sinon.spy(data.nogCatalog, 'callUpdateCatalog');
        btn.click();
        sinon.assert.calledOnce(save);
        done();
      });
    });
  });
  describe('Template nogCatalogTools', function () {
    it('Renders all log messages', function (done) {
      const { ctxCatalogTools } = createFakeGlobals();
      withDiv((el) => {
        Blaze.renderWithData(Template.nogCatalogTools, ctxCatalogTools, el);
        const lis = findElement(el, 'li.t-log-msg');
        const logs = ctxCatalogTools.logMessages();
        const length = logs.msgs.length + Object.keys(logs.diffs).length;
        expect(lis.length).to.eql(length);
        done();
      });
    });
  });
});
