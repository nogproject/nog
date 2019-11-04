resolve = require('path').resolve;
var time = 1000;
var file = 'a.txt';

module.exports = {

  before : function(client) {
    client
        .url('http://127.0.0.1:3000')
        .pause(1000);
  },

  after : function(client) {
    client.end();
  },

  'On start page': function(client) {
    client.expect.element('.js-secret-code').to.be.visible.before(time);
    client
        .setValue('.js-secret-code', 'Secret&Code:For3Testing!');

    client.expect.element('.js-create-repo').to.be.visible.before(time);
    client
        .submitForm('.js-create-repo');

    client.expect.element('.js-upload').to.be.visible.before(time);
    client
        .click('.js-upload');

    client.expect.element('input[type="file"]').to.be.present.before(time);
    client
        .setValue('input[type="file"]', resolve(__dirname + '/' + file));
    client.expect.element('.progress').to.be.visible.before(time);

    client.expect.element('button[data-dismiss="modal"]')
            .to.be.visible.before(time);
    client
        .click('button[data-dismiss="modal"]');

    client.expect.element('tr').to.be.visible.before(time);
    client.expect.element('tr[class="info"]').to.be.not.present;

    client.expect.element('.nog-files-entry-name-td')
            .to.be.visible.before(time);
    client.expect.element('.nog-files-entry-name-td').text.to.contain(file);

    client
        .click('tr');
    client.expect.element('tr').to.be.visible.before(time);
    client.expect.element('tr[class="info"]').to.be.visible.before(time);
    client.expect.element('.js-download')
            .to.be.visible.before(time);
    client.expect.element('.js-download[disabled=true]')
            .to.be.not.present.before(time);

  }
};

