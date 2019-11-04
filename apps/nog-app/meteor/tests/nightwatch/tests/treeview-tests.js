/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable object-shorthand */
/* eslint quotes: ["error", "single", { "allowTemplateLiterals": true }] */

require('coffee-script/register')
const flags = require('nog-test-flags').flags
const resolve = require('path').resolve;
const deleteRepo = require('../commands/deleteRepo').deleteRepo;

const file = 'a.txt';
const file2 = 'b.txt';
const dir = 'folder';
let username = '';
const reponame = `test-${Math.random().toString(36).slice(2)}`;
const reponame2 = `test-${Math.random().toString(36).slice(2)}`;
let maxWaitMs = 3000;
let waitMs = 1000;

const multiplier = process.env.NOG_TEST_WAITINGTIME_MULTIPLIER;
if (multiplier) {
  maxWaitMs = maxWaitMs * multiplier;
  waitMs = waitMs * multiplier;
}


function tagItemWithTextPresent(client, sel, string) {
  let present = false;
  client.elements('css selector', sel, function (elements) {
    elements.value.forEach(function (elt, idx, arr) {
      client.elementIdText(elt.ELEMENT, function (text) {
        if (text.value === string) {
          present = true;
        }
        if (idx + 1 === arr.length) {
          this.assert.ok(present === true,
            `Expected item of <${sel}> with text "${string}" to be present.`
          );
        }
      });
    });
  });
}

function tagItemWithTextNotPresent(client, sel, string) {
  let notPresent = true;
  client.elements('css selector', sel, function (elements) {
    elements.value.forEach(function (elt, idx, arr) {
      client.elementIdText(elt.ELEMENT, function (text) {
        if (text.value === string) {
          notPresent = false;
        }
        if (idx + 1 === arr.length) {
          this.assert.ok(notPresent === true,
            `Expected item of <${sel}> with text "${string}" to be NOT` +
            `present.`
          );
        }
      });
    });
  });
}

function clickTagItemWithText(client, sel, string) {
  let clicked = false;
  client.elements('css selector', sel, function (elements) {
    elements.value.forEach(function (elt, idx, arr) {
      client.elementIdText(elt.ELEMENT, function (text) {
        if (text.value === string) {
          clicked = true;
          client.click(`${sel}:nth-child(${idx + 1}) a`);
        }
        if (idx + 1 === arr.length) {
          this.assert.ok(clicked === true,
            `Clicked on <${sel}:nth-child(${idx + 1}) a>.`
          );
        }
      });
    });
  });
}

module.exports = {
  // Skip entire test module if flaky is disabled, since we repeatedly observed
  // timeouts and could not immediately explain why.
  '@disabled': flags.skipFlaky(),

  before: function (client) {
    console.log(
      `The tests run with 'maxWaitMs' = ${maxWaitMs} ms and 'waitM' = ` +
      `${waitMs} ms.`
    );
    client
      .url('http://127.0.0.1:3000')
      .expect.element('.t-login-user').to.be.visible.before(1000);
    client
      .click('.t-login-user')
      .pause(waitMs);
    client.getText('.t-current-user', function (result) {
      username = result.value;
    });
  },

  after: function (client) {
    deleteRepo(client, username, reponame, maxWaitMs);
    deleteRepo(client, username, reponame2, maxWaitMs);
    client.end();
  },

  '[tree view tests] Logged in': function (client) {
    client.expect.element('.t-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').text.to.contain(username);
  },

  '[tree view tests] Create workspace repo': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-new-repo').to.be.visible.before(maxWaitMs);

    client.click('.t-new-repo');
    client.expect.element('.t-create-repo').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-type-workspace').to.be.visible.before(maxWaitMs);

    client
      .click('.t-type-workspace')
      .expect.element('.t-type-workspace').to.be.selected.before(maxWaitMs);

    client
      .setValue('.t-set-repo-name', reponame)
      .click('.t-create')
      .url(`http://127.0.0.1:3000/${username}/${reponame}/tree/master`)
      .expect.element('.t-tree').to.be.visible.before(maxWaitMs);
  },

  '[tree view tests] Upload file': function (client) {
    client
      .url(`http://127.0.0.1:3000/${username}/${reponame}/tree/master`);
    client.expect.element('tr').to.be.visible.before(maxWaitMs);

    tagItemWithTextPresent(client, 'td', 'datalist');
    clickTagItemWithText(client, 'td', 'datalist');
    client.expect.element('.js-upload-files').to.be.visible.before(maxWaitMs);
    client.expect.element('input[type="file"]')
      .to.be.present.before(maxWaitMs);
    client
      .setValue('input[type="file"]', resolve(`${__dirname}/${file}`));
    client.waitForElementPresent('div.progress-bar-success', 10 * maxWaitMs);

    client.expect.element('.nog-error-display')
      .to.be.not.present.after(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
  },

  '[tree view tests] Add data to a new datalist': function (client) {
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    client.expect.element('.nog-tree-entry-checkbox')
      .to.be.visible.before(maxWaitMs);

    client.click('.nog-tree-entry-checkbox');
    client.expect.element('.t-add-data').to.be.visible.before(maxWaitMs);

    client.click('.t-add-data');
    client.expect.element('.t-add-data-menu').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-new-datalist')
      .to.be.visible.before(2 * maxWaitMs);

    client.click('.js-new-datalist');
    client.expect.element('.js-new-datalist-modal')
      .to.be.visible.before(2 * maxWaitMs);
    client.expect.element('.js-new-repo-name').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-create-and-add[disabled=true]')
      .to.be.visible.before(maxWaitMs);

    client.setValue('.js-new-repo-name', reponame2);
    client.expect.element('.js-create-and-add')
      .to.be.visible.before(maxWaitMs);
    client.expect.element('.js-create-and-add[disabled=true]')
      .to.be.not.present.before(maxWaitMs);
    client.click('.js-create-and-add');
    client.waitForElementNotVisible('.js-new-datalist-modal', 4 * maxWaitMs);
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(2 * maxWaitMs);

    client
      .url(`http://127.0.0.1:3000/${username}/${reponame}/tree/master`)
      .expect.element('.t-tree').to.be.visible.before(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', 'datalist');
    clickTagItemWithText(client, 'td', 'datalist');
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
  },

  '[tree view tests] Add data to another datalist': function (client) {
    client
      .url(`http://127.0.0.1:3000/${username}/${reponame}/tree/master`)
      .expect.element('.t-tree').to.be.visible.before(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', 'datalist');

    clickTagItemWithText(client, 'td', 'datalist');
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    client.expect.element('.nog-tree-entry-checkbox')
      .to.be.visible.before(maxWaitMs);

    client.click('.nog-tree-entry-checkbox');
    client.expect.element('.t-add-data').to.be.visible.before(maxWaitMs);

    client.click('.t-add-data');
    client.expect.element('.t-add-data-menu').to.be.visible.before(maxWaitMs);
    client.pause(10 * waitMs);
    tagItemWithTextPresent(client, '.t-add-data-menu li', reponame2);

    clickTagItemWithText(client, '.t-add-data-menu li', reponame2);
    client.waitForElementNotVisible('.t-add-data-menu', 2 * maxWaitMs);
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(maxWaitMs);

    client
      .url(`http://127.0.0.1:3000/${username}/${reponame2}/tree/master`)
      .expect.element('.t-tree').to.be.visible.before(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', 'datalist');
    clickTagItemWithText(client, 'td', 'datalist');
    client.expect.element('tr').to.be.visible.before(maxWaitMs);
    client.assert.containsText('tr:nth-child(1)', file);
    client.assert.containsText('tr:nth-child(2)', file);
  },

  '[tree view tests] Add folder': function (client) {
    client
      .url(`http://127.0.0.1:3000/${username}/${reponame}/tree/master`)
      .expect.element('.t-tree').to.be.visible.before(2 * maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);

    tagItemWithTextPresent(client, 'td', 'datalist');
    clickTagItemWithText(client, 'td', 'datalist');
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);

    client.expect.element('.nog-tree-entry-checkbox')
      .to.be.visible.before(maxWaitMs);
    client.click('.nog-tree-entry-checkbox');
    client.expect.element('.js-addFolder-start')
      .to.be.visible.before(maxWaitMs);
    client.click('.js-addFolder-start');

    client.expect.element('.js-addFolder-name')
      .to.be.present.before(2 * maxWaitMs);
    client.expect.element('.js-addFolder-complete[disabled=true]')
      .to.be.visible.before(maxWaitMs);

    client.setValue('.js-addFolder-name', dir);

    client.expect.element('.js-addFolder-complete[disabled=true]')
      .to.be.not.present.before(maxWaitMs);
    client.click('.js-addFolder-complete');
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(maxWaitMs);
    client.waitForElementNotVisible('.js-addFolder-name', maxWaitMs);

    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    tagItemWithTextPresent(client, 'td', dir);
  },

  '[tree view tests] Rename file': function (client) {
    client
      .url(`http://127.0.0.1:3000/${username}/${reponame}/tree/master` +
        `/datalist`)
      .expect.element('.t-tree').to.be.visible.before(2 * maxWaitMs);

    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    client.expect.element('.nog-tree-entry-checkbox')
      .to.be.visible.before(maxWaitMs);
    client.click('.nog-tree-entry-checkbox');
    client.expect.element('.js-rename[disabled="disabled"]')
      .to.be.visible.before(maxWaitMs);

    client.click('.js-name-val');
    client.clearValue('.js-name-val');
    client.setValue('.js-name-val', file2);
    client.expect.element('.js-rename').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-rename[disabled="disabled"]')
      .to.be.not.present.after(maxWaitMs);

    client.click('.js-rename');
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    client.pause(waitMs);

    // XXX Reload page as a workaround for an UI update issue with Meteor
    // 1.3.3.  Instead of `b.txt`, the UI would display `b.txtb.txt` without
    // reload, although the name has been correctly changed and appears
    // correctly after reload.  The test workaround should be sufficient, since
    // we do not care much about the tree view.

    client
      .url(`http://127.0.0.1:3000/${username}/${reponame}/tree/master` +
        `/datalist`)
      .expect.element('.t-tree').to.be.visible.before(2 * maxWaitMs);

    tagItemWithTextPresent(client, 'td', file2);
  },

  '[tree view tests] Delete file': function (client) {
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file2);
    client.expect.element('.nog-tree-entry-checkbox')
      .to.be.visible.before(maxWaitMs);
    client.click('.nog-tree-entry-checkbox');
    client.expect.element('.js-delete').to.be.visible.before(maxWaitMs);

    client.click('.js-delete');
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    client.pause(waitMs);
    tagItemWithTextNotPresent(client, 'td', file2);
  },
};
