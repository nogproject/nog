/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable object-shorthand */
/* eslint quotes: ["error", "single", { "allowTemplateLiterals": true }] */

const resolve = require('path').resolve;
const deleteRepo = require('../commands/deleteRepo').deleteRepo;

const file = 'a.txt';
const file2 = 'b.txt';
const dir = 'folder';
let username = '';
const reponame = `test-${Math.random().toString(36).slice(2)}`;
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
          this.assert.ok(present === true, `Expected item of < ${sel} ` +
            `> with text "${string}" to be present.`);
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
            `Expected item of <${sel}> with text "${string}" to be NOT ` +
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
          client.click(`${sel}:nth-child(${idx + 1})`);
        }
        if (idx + 1 === arr.length) {
          this.assert.ok(clicked === true,
            `Clicked on < + ${sel}:nth-child(${idx + 1})>.`
          );
        }
      });
    });
  });
}

module.exports = {
  before: function (client) {
    console.log(
      `The tests run with 'maxWaitMs' = ${maxWaitMs} ms and 'waitM' = ` +
      `${waitMs} ms.`
    );
    client
      .url('http://127.0.0.1:3000')
      .expect.element('.t-login-user').to.be.visible.before(waitMs);
    client
      .click('.t-login-user')
      .pause(waitMs);
    client.getText('.t-current-user', function (result) {
      username = result.value;
    });
  },

  after: function (client) {
    deleteRepo(client, username, reponame, maxWaitMs);
    client.end();
  },

  '[file view tests] Logged in': function (client) {
    client.expect.element('.t-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').text.to.contain(username);
  },

  '[file view tests] Create file repo': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-new-repo').to.be.visible.before(maxWaitMs);

    client.click('.t-new-repo');
    client.expect.element('.t-create-repo').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-type-files').to.be.visible.before(maxWaitMs);

    client
      .click('.t-type-files')
      .expect.element('.t-type-files').to.be.selected.before(maxWaitMs);

    client
      .setValue('.t-set-repo-name', reponame)
      .click('.t-create')
      .expect.element('.t-files').to.be.visible.before(maxWaitMs);
  },

  '[file view tests] Upload file to file repo': function (client) {
    client.expect.element('.js-upload').to.be.visible.before(maxWaitMs);
    client
      .click('.js-upload');

    client.expect.element('.js-upload-files').to.be.visible.before(maxWaitMs);
    client.expect.element('input[type="file"]')
      .to.be.present.before(maxWaitMs);
    client
      .setValue('input[type="file"]', resolve(`${__dirname}/${file}`));
    client.waitForElementPresent('div.progress-bar-success', 10 * maxWaitMs);

    client.expect.element('.t-close-modal').to.be.visible.before(maxWaitMs);
    client.pause(2 * waitMs);
    client.click('.t-close-modal');
    client.expect.element('.js-upload-files')
      .to.be.not.present.before(maxWaitMs);

    client.expect.element('tr').to.be.visible.before(maxWaitMs);
    client.expect.element('tr[class="info"]')
      .to.be.not.present.before(maxWaitMs);

    client.expect.element('.nog-files-entry-name-td')
      .to.be.visible.before(maxWaitMs);
    client.expect.element('.nog-files-entry-name-td').text.to.contain(file);
  },

  '[file view tests] Add folder': function (client) {
    client.expect.element('.js-new-folder').to.be.visible.before(maxWaitMs);
    client
      .click('.js-new-folder');

    client.expect.element('.js-folder-name').to.be.present.before(maxWaitMs);
    client
      .setValue('.js-folder-name', dir);
    client.expect.element('.js-create').to.be.visible.before(maxWaitMs);
    client
    .click('.js-create');
    client.expect.element('.js-folder-name')
      .to.be.not.present.before(maxWaitMs);
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(waitMs);

    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    tagItemWithTextPresent(client, 'td', dir);
  },

  '[file view tests] Move file within repo': function (client) {
    client.expect.element('.t-move-in-repo[disabled=true]')
      .to.be.present.before(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    clickTagItemWithText(client, 'td', file);
    client.expect.element('.t-move-in-repo').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-move-in-repo[disabled=true]')
      .to.be.not.present.before(maxWaitMs);

    client.click('.t-move-in-repo');

    tagItemWithTextPresent(client, '.js-move-to', `   ./${dir}`);
    clickTagItemWithText(client, '.js-move-to', `   ./${dir}`);
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(waitMs);
    client.url(`http://127.0.0.1:3000/${username}/${reponame}/files/${dir}`);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
  },

  '[file view tests] Copy file within repo': function (client) {
    client.url(`http://127.0.0.1:3000/${username}/${reponame}/files/${dir}`);
    client.expect.element('.t-copy-in-repo[disabled=true]')
      .to.be.present.before(maxWaitMs);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    clickTagItemWithText(client, 'td', file);
    client.expect.element('.t-copy-in-repo').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-copy-in-repo[disabled=true]')
      .to.be.not.present.before(maxWaitMs);

    client.click('.t-copy-in-repo');

    tagItemWithTextPresent(client, '.js-copy-to', `/${reponame}/`);
    clickTagItemWithText(client, '.js-copy-to', `/${reponame}/`);
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(waitMs);
    client
      .url(`http://127.0.0.1:3000/${username}/${reponame}/files`);
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
  },

  '[file view tests] Rename file': function (client) {
    client.expect.element('.js-start-rename[disabled=true]')
      .to.be.present.before(maxWaitMs);

    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file);
    clickTagItemWithText(client, 'td', file);

    client.expect.element('.js-start-rename').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-start-rename[disabled=true]')
      .to.be.not.present.before(maxWaitMs);

    client.click('.js-start-rename');
    client.expect.element('.js-rename-modal').to.be.visible.before(maxWaitMs);
    client.expect.element('input[type="text"]')
      .to.be.present.before(maxWaitMs);
    client.clearValue('input[type=text]');
    client.setValue('input[type="text"]', file2);
    client.expect.element('.js-rename').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-rename[disabled=true]')
      .to.be.not.present.before(maxWaitMs);
    client.click('.js-rename');

    client.expect.element('.js-rename-modal')
      .to.be.not.present.before(maxWaitMs);

    client.expect.element('td').to.be.visible.before(maxWaitMs);
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(waitMs);
    tagItemWithTextPresent(client, 'td', file2);
  },

  '[file view tests] Delete file': function (client) {
    client.expect.element('.js-delete[disabled=true]')
      .to.be.present.before(maxWaitMs);

    client.expect.element('td').to.be.visible.before(maxWaitMs);
    tagItemWithTextPresent(client, 'td', file2);
    clickTagItemWithText(client, 'td', file2);

    client.expect.element('.js-delete').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-delete[disabled=true]')
      .to.be.not.present.before(maxWaitMs);

    client.click('.js-delete');
    client.expect.element('td').to.be.visible.before(maxWaitMs);
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(waitMs);
    client.pause(waitMs);
    tagItemWithTextNotPresent(client, 'td', file2);
  },
};
