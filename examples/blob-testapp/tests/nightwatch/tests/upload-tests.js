resolve = require('path').resolve

require('coffee-script/register')
flags = require('nog-test-flags').flags

module.exports = {
  "File upload reports success with the correct file size.": function (client) {
    if (!flags.useRealAws()) {
      client.end();
      return;
    }
    client
      .url("http://127.0.0.1:3000")
      .waitForElementVisible("button.login-user-button", 1000)
      .click("button.login-user-button")
      .expect.element(".t-current-user-name")
      .text.to.equal("__testing__user").before(500);
    client.waitForElementVisible("input#files", 1000)
      .setValue('input#files', resolve(__dirname + '/a.txt'))
      .waitForElementVisible("div.progress-bar-success", 5000)
      .assert.containsText("div.progress-bar", "100%")
      .assert.containsText("div.progress-bar", "2 Bytes")
      .end();
  }
};
