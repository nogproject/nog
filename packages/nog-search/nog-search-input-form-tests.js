/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */
/* eslint-disable max-len */

import { Blaze } from 'meteor/blaze';
import { Template } from 'meteor/templating';
import { expect } from 'chai';
import sinon from 'sinon';
import { $ } from 'meteor/jquery';
import './nog-search-input-form.html';


const SEARCH_INPUT = '.js-search-input';
const SEARCH_COMPLETION = '.t-search-completion';
const COMPLETION_LIST = '.js-completion-list';
const COMPLETION_ITEM = '.js-completion-item';

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


function createFakeRouter() {
  const repo = {
    ownerName: 'fakeOwner',
    repoName: 'fakeRepo',
  };

  const queryParams = {};

  const router = {
    getParam(name) { return repo[name]; },
    getRouteName() { return 'fakeRouteName'; },
    current() { return { queryParams }; },
    setQueryParams(p) {
      queryParams.q = p.q;
    },
  };

  return router;
}


describe('nog-search', function () {
  describe('Template nogSearchInputForm', function () {
    function simulateTyping(el, str) {
      findElement(el, SEARCH_INPUT).val(str);
      findElement(el, SEARCH_INPUT).keyup();
    }

    function simulatePressEnter(el) {
      // eslint-disable-next-line
      const e = $.Event('keyup');
      e.which = 13;
      findElement(el, SEARCH_INPUT).trigger(e);
    }

    const testStr = 'test';

    it('with updateOnEnter=false, invokes onUpdateInput() on every keyup', function (done) {
      let called = null;
      const data = {
        inputFormLabel: 'Filter',
        updateOnEnter: false,
        onUpdateInput(str) {
          called = str;
        },
        router: createFakeRouter(),
      };

      withDiv((el) => {
        Blaze.renderWithData(Template.nogSearchInputForm, data, el);
        expect(findElement(el, SEARCH_INPUT).length).to.eql(1);
        simulateTyping(el, testStr);
        expect(called).to.eql(testStr);
        done();
      });
    });

    it('with updateOnEnter=true, invokes onUpdateInput() only on ENTER', function (done) {
      let called = null;
      const data = {
        inputFormLabel: 'Filter',
        updateOnEnter: true,
        onUpdateInput(str) {
          called = str;
        },
        router: createFakeRouter(),
      };

      withDiv((el) => {
        Blaze.renderWithData(Template.nogSearchInputForm, data, el);
        expect(findElement(el, SEARCH_INPUT).length).to.eql(1);
        simulateTyping(el, testStr);
        expect(called).to.eql('');
        simulatePressEnter(el);
        expect(called).to.eql(testStr);
        done();
      });
    });
  });

  describe('Autocompletion', function () {
    const testStr = 'test';
    const fooStr = 'foo';
    const bStr = 'b';
    const list = ['foo', 'bar', 'baz'];
    function onChooseItem() {}

    it('With ["foo", "bar", "baz"] & input = "b": displays 2 items', function (done) {
      withDiv((el) => {
        const ctx = {
          completionList: list,
          completionString: bStr,
          onChooseItem,
        };
        Blaze.renderWithData(Template.searchAutocomplete, ctx, el);
        expect(findElement(el, SEARCH_COMPLETION).length).to.eql(1);
        expect(findElement(el, COMPLETION_LIST).length).to.eql(1);
        expect(findElement(el, COMPLETION_ITEM).length).to.eql(2);
        done();
      });
    });

    it('With ["foo", "bar", "baz"] & input = "foo": displays 1 item', function (done) {
      withDiv((el) => {
        const ctx = {
          completionList: list,
          completionString: fooStr,
          onChooseItem,
        };
        Blaze.renderWithData(Template.searchAutocomplete, ctx, el);
        expect(findElement(el, SEARCH_COMPLETION).length).to.eql(1);
        expect(findElement(el, COMPLETION_LIST).length).to.eql(1);
        expect(findElement(el, COMPLETION_ITEM).length).to.eql(1);
        done();
      });
    });

    it('With ["foo", "bar", "baz"] & input = "test": displays no item', function (done) {
      withDiv((el) => {
        const ctx = {
          completionList: list,
          completionString: testStr,
          onChooseItem,
        };
        Blaze.renderWithData(Template.searchAutocomplete, ctx, el);
        expect(findElement(el, SEARCH_COMPLETION).length).to.eql(1);
        expect(findElement(el, COMPLETION_LIST).length).to.eql(0);
        expect(findElement(el, COMPLETION_ITEM).length).to.eql(0);
        done();
      });
    });

    it('With ["foo", "bar", "baz"] & input = "": displays no item', function (done) {
      withDiv((el) => {
        const ctx = {
          completionList: list,
          completionString: '',
          onChooseItem,
        };
        Blaze.renderWithData(Template.searchAutocomplete, ctx, el);
        expect(findElement(el, SEARCH_COMPLETION).length).to.eql(1);
        expect(findElement(el, COMPLETION_LIST).length).to.eql(0);
        expect(findElement(el, COMPLETION_ITEM).length).to.eql(0);
        done();
      });
    });

    it('With empty list: displays no list', function (done) {
      withDiv((el) => {
        const ctx = {
          completionList: [],
          completionString: bStr,
          onChooseItem,
        };
        Blaze.renderWithData(Template.searchAutocomplete, ctx, el);
        expect(findElement(el, SEARCH_COMPLETION).length).to.eql(1);
        expect(findElement(el, COMPLETION_LIST).length).to.eql(0);
        expect(findElement(el, COMPLETION_ITEM).length).to.eql(0);
        done();
      });
    });

    it('Click on item triggers "onChooseItem()"', function (done) {
      withDiv((el) => {
        const ctx = {
          completionList: list,
          completionString: bStr,
          onChooseItem,
        };
        Blaze.renderWithData(Template.searchAutocomplete, ctx, el);
        const items = findElement(el, COMPLETION_ITEM);
        expect(items.length).to.eql(2);
        const save = sinon.spy(ctx, 'onChooseItem');
        findElement(el, COMPLETION_ITEM)[0].click();
        sinon.assert.calledOnce(save);
        done();
      });
    });
  });
});
