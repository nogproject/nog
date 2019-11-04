/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */
/* eslint-disable max-len */

import { $ } from 'meteor/jquery';
import { Blaze } from 'meteor/blaze';
import { Template } from 'meteor/templating';
import { expect } from 'chai';
import sinon from 'sinon';
import './nog-repo-toolbar.js';


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

function createFakeContext() {
  const repo = {
    ownerName: 'fakeOwner',
    repoName: 'fakeRepo',
  };

  const router = {
    getParam(name) { return repo[name]; },
    getRouteName() { return 'fakeRouteName'; },
  };

  const viewerInfo = {
    ownerName: repo.ownerName,
    repoName: repo.repoName,
    type: 'tree',
    treePath: '',
    iskindWorkspace: false,
    iskindCatalog: false,
  };

  const ctxPresenter = {
    router,
    nogCatalog: {
      testAccess() { return true; },
      volumeRegistry: {},
    },
    nogContent: {
      repos: {
        findOne() {
          return {};
        },
      },
      call: {
        forkRepo() {
          return {};
        },
      },
    },
    meteorUser() { return { username: 'fakeUser' }; },
    viewerInfo,
    ownerName: repo.ownerName,
    repoName: repo.repoName,
  };

  const ctxForkedFrom = {
    forkedFrom() {
      return {
        owner: repo.ownerName,
        name: repo.repoName,
      };
    },
    routeName() { return 'fakeRouteName'; },
  };

  const ctxToolbar = {
    router: {
      getParam(name) { return repo[name]; },
      getRouteName() { return 'fakeRouteName'; },
    },
    testAccess() { return true; },
    meteorUser() { return { username: 'fakeUser' }; },
    onForkRepo() { return {}; },
    forkAction() { return false; },
  };

  return {
    ctxPresenter,
    ctxForkedFrom,
    ctxToolbar,
  };
}


// XXX Some tests fail with Meteor 1.6.1 with `Error: No such function: part`.
// The reason is unclear.
describe('nog-repo-toolbar', function () {
  describe('Template nogRepoTopBarPresenter', function () {
    it('Presenter template exists', function (done) {
      withDiv((el) => {
        const { ctxPresenter } = createFakeContext();
        Blaze.renderWithData(
          Template.nogRepoTopBarPresenter, ctxPresenter, el
        );
        expect(findElement(el, '.t-repo-top-bar-presenter').length).to.eql(1);
        done();
      });
    });
    it('Presenter renders template "viewerButtons"', function (done) {
      withDiv((el) => {
        const { ctxPresenter } = createFakeContext();
        Blaze.renderWithData(
          Template.nogRepoTopBarPresenter, ctxPresenter, el
        );
        expect(findElement(el, '.t-viewer-buttons').length).to.eql(1);
        done();
      });
    });
    it('Presenter renders template "repoToolbar"', function (done) {
      withDiv((el) => {
        const { ctxPresenter } = createFakeContext();
        Blaze.renderWithData(
          Template.nogRepoTopBarPresenter, ctxPresenter, el
        );
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        done();
      });
    });
    it('Presenter renders template "forkedFrom"', function (done) {
      withDiv((el) => {
        const { ctxPresenter } = createFakeContext();
        Blaze.renderWithData(
          Template.nogRepoTopBarPresenter, ctxPresenter, el
        );
        expect(findElement(el, '.t-forked-from').length).to.eql(1);
        done();
      });
    });
    it('Clicking the fork button triggers method call', function (done) {
      withDiv((el) => {
        const { ctxPresenter } = createFakeContext();
        Blaze.renderWithData(
          Template.nogRepoTopBarPresenter, ctxPresenter, el
        );
        expect(findElement(el, '.t-repo-top-bar-presenter').length).to.eql(1);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        const btn = findElement(el, '.js-fork-repo');
        expect(btn.length).to.eql(1);
        const save = sinon.spy(ctxPresenter.nogContent.call, 'forkRepo');
        btn.click();
        sinon.assert.calledOnce(save);
        done();
      });
    });
  });
  describe('Template forkedFrom', function () {
    it('Template exists', function (done) {
      withDiv((el) => {
        const { ctxForkedFrom } = createFakeContext();
        Blaze.renderWithData(Template.forkedFrom, ctxForkedFrom, el);
        expect(findElement(el, '.t-forked-from').length).to.eql(1);
        expect(findElement(el, '.t-forked-from-info').length).to.eql(1);
        done();
      });
    });
    it('Template displays forked-from repo', function (done) {
      withDiv((el) => {
        const { ctxForkedFrom } = createFakeContext();
        const owner = ctxForkedFrom.forkedFrom().owner;
        const name = ctxForkedFrom.forkedFrom().name;
        const textForkedFrom = `${owner}/${name}`;
        Blaze.renderWithData(Template.forkedFrom, ctxForkedFrom, el);
        expect(findElement(el, 'a').text()).to.eql(textForkedFrom);
        done();
      });
    });
    it('Template does not exist with empty "forkedFrom" ctx', function (done) {
      withDiv((el) => {
        const { ctxForkedFrom } = createFakeContext();
        ctxForkedFrom.forkedFrom = function () { return null; };
        Blaze.renderWithData(Template.forkedFrom, ctxForkedFrom, el);
        expect(findElement(el, '.t-forked-from').length).to.eql(1);
        expect(findElement(el, '.t-forked-from-info').length).to.eql(0);
        done();
      });
    });
    it('Template provides forked-from route', function (done) {
      withDiv((el) => {
        const { ctxForkedFrom } = createFakeContext();
        const owner = ctxForkedFrom.forkedFrom().owner;
        const name = ctxForkedFrom.forkedFrom().name;
        const routeName = ctxForkedFrom.routeName();
        const routeForkedFrom = `/${owner}/${name}/${routeName}`;
        Blaze.renderWithData(Template.forkedFrom, ctxForkedFrom, el);
        expect(findElement(el, 'a').attr('href')).to.eql(routeForkedFrom);
        done();
      });
    });
  });
  describe('Template nogToolbar', function () {
    it('Settings button with access', function (done) {
      withDiv((el) => {
        const { ctxToolbar } = createFakeContext();
        Blaze.renderWithData(Template.repoToolbar, ctxToolbar, el);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        expect(findElement(el, '.t-repo-tool-bar-settings').length).to.eql(1);
        done();
      });
    });
    it('No settings button without access', function (done) {
      withDiv((el) => {
        const { ctxToolbar } = createFakeContext();
        ctxToolbar.testAccess = function () { return false; };
        Blaze.renderWithData(Template.repoToolbar, ctxToolbar, el);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        expect(findElement(el, '.t-repo-tool-bar-settings').length).to.eql(0);
        done();
      });
    });
    it('Fork button with access', function (done) {
      withDiv((el) => {
        const { ctxToolbar } = createFakeContext();
        Blaze.renderWithData(Template.repoToolbar, ctxToolbar, el);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        expect(findElement(el, '.t-repo-tool-bar-fork').length).to.eql(1);
        done();
      });
    });
    it('No fork button without access', function (done) {
      withDiv((el) => {
        const { ctxToolbar } = createFakeContext();
        ctxToolbar.testAccess = function () { return false; };
        Blaze.renderWithData(Template.repoToolbar, ctxToolbar, el);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        expect(findElement(el, '.t-repo-tool-bar-fork').length).to.eql(0);
        done();
      });
    });
    it('Sharing button with access', function (done) {
      withDiv((el) => {
        const { ctxToolbar } = createFakeContext();
        ctxToolbar.meteorUser = function () {
          return { username: 'fakeOwner' };
        };
        Blaze.renderWithData(Template.repoToolbar, ctxToolbar, el);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        expect(findElement(el, '.t-repo-tool-bar-sharing').length).to.eql(1);
        done();
      });
    });
    it('No sharing button without access', function (done) {
      withDiv((el) => {
        const { ctxToolbar } = createFakeContext();
        Blaze.renderWithData(Template.repoToolbar, ctxToolbar, el);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        expect(findElement(el, '.t-repo-tool-bar-sharing').length).to.eql(0);
        done();
      });
    });
    it('Renders "Forking..." note during forking', function (done) {
      withDiv((el) => {
        const { ctxToolbar } = createFakeContext();
        ctxToolbar.forkAction = function () { return true; };
        Blaze.renderWithData(Template.repoToolbar, ctxToolbar, el);
        expect(findElement(el, '.t-repo-tool-bar').length).to.eql(1);
        expect(findElement(el, '.js-fork-repo').length).to.eql(0);
        expect(
          findElement(el, '.t-repo-tool-bar-fork-active').length
        ).to.eql(1);
        done();
      });
    });
  });
});
