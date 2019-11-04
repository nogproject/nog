/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable object-shorthand */
/* eslint quotes: ["error", "single", { "allowTemplateLiterals": true }] */

const resolve = require('path').resolve;
const deleteRepo = require('../commands/deleteRepo').deleteRepo;

const file = 'a.txt';
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


module.exports = {
  before: function (client) {
    console.log(
      `The tests run with 'maxWaitMs' = ${maxWaitMs} ms and 'waitM' = ` +
      `${waitMs} ms.`
    );
    client
      .url('http://127.0.0.1:3000')
      .expect.element('.t-login-user').to.be.visible.before(maxWaitMs);
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

  '[workspace tests] Logged in': function (client) {
    client.expect.element('.t-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').text.to.contain(username);
  },

  '[workspace tests] Create file repo': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-new-repo').to.be.visible.before(maxWaitMs);

    client.click('.t-new-repo');
    client.expect.element('.t-create-repo').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-type-files').to.be.visible.before(maxWaitMs);

    client
      .click('.t-type-files')
      .expect.element('.t-type-files').to.be.selected.after(waitMs);

    client
      .setValue('.t-set-repo-name', reponame2)
      .click('.t-create')
      .expect.element('.t-files').to.be.visible.before(maxWaitMs);
  },

  '[workspace tests] Upload file to file repo': function (client) {
    client.expect.element('.js-upload').to.be.visible.before(maxWaitMs);
    client
      .click('.js-upload');

    client.expect.element('input[type="file"]')
      .to.be.present.before(maxWaitMs);
    client
      .setValue('input[type="file"]', resolve(`${__dirname}/${file}`));
    client.expect.element('div.progress-bar-success')
      .to.be.visible.before(maxWaitMs);

    client.expect.element('.t-close-modal')
      .to.be.visible.before(maxWaitMs);
    client
      .click('.t-close-modal');

    client.expect.element('tr').to.be.visible.before(maxWaitMs);
    client.expect.element('tr[class="info"]')
      .to.be.not.present.after(waitMs);

    client.expect.element('.nog-files-entry-name-td')
      .to.be.visible.before(maxWaitMs);
    client.expect.element('.nog-files-entry-name-td').text.to.contain(file);
  },

  '[workspace tests] Create workspace repo': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-new-repo').to.be.visible.before(maxWaitMs);

    client.click('.t-new-repo');
    client.expect.element('.t-create-repo').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-type-workspace').to.be.visible.before(maxWaitMs);

    client
      .click('.t-type-workspace')
      .expect.element('.t-type-workspace').to.be.selected.after(waitMs);

    client
      .setValue('.t-set-repo-name', reponame)
      .click('.t-create')
      .expect.element('.t-workspace').to.be.visible.before(maxWaitMs);
  },

  '[workspace tests] NogModal mode when adding data': function (client) {
    client.expect.element('.t-workspace').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-browse-add-files')
      .to.be.visible.before(maxWaitMs);

    client.click('.t-browse-add-files');
    client.expect.element('.t-nogmodal').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-nogmodal-back').to.be.visible.before(maxWaitMs);

    client.click('.t-nogmodal-back');
    client.expect.element('.t-workspace').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-nogmodal').to.be.not.present.after(waitMs);
    client.expect.element('.t-browse-search').to.be.visible.before(maxWaitMs);

    client.click('.t-browse-search');
    client.expect.element('.t-nogmodal').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-nogmodal-back').to.be.visible.before(maxWaitMs);

    client.click('.t-nogmodal-back');
    client.expect.element('.t-workspace').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-nogmodal').to.be.not.present.after(waitMs);
  },

  '[workspace tests] Add data from file repo': function (client) {
    client.expect.element('.t-workspace').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-browse-add-files')
      .to.be.visible.before(maxWaitMs);
    client.click('.t-browse-add-files');

    client.expect.element(`a[href*=${reponame2}]`)
      .to.be.visible.before(maxWaitMs);
    client.click(`a[href*=${reponame2}]`);

    client.expect.element('tr').to.be.visible.before(maxWaitMs);
    client.expect.element('tr[class="info"]')
      .to.be.not.present.after(waitMs);
    client.expect.element('.nog-files-entry-name-td').text.to.contain(file);
    client
      .click('tr');

    client.expect.element('.js-add-to-target')
      .to.be.visible.before(maxWaitMs);
    client.expect.element('.js-add-to-target[disabled=true]')
      .to.be.not.present.after(waitMs);
    client.click('.js-add-to-target');
    client.expect.element('.nog-error-display')
      .to.be.not.present.after(waitMs);

    client.click('.t-nogmodal-back');
    client.expect.element('.t-workspace').to.be.visible.before(maxWaitMs);

    client.expect.element('tr').to.be.visible.before(maxWaitMs);
    client.expect.element('tr').text.to.contain(file);
  },
};
