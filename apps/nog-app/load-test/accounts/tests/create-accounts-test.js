resolve = require('path').resolve;

fs = require('fs');
readFileSync = fs.readFileSync;
unlinkSync = fs.unlinkSync;

crypto = require('crypto');
randomBytes = crypto.randomBytes;

// We observed problems when connecting to as SSL replica set with the 2.1
// driver.  Version 1.4.35 worked.  Meteor also uses 1.4.x.

MongoClient = require('mongodb').MongoClient;

config = process.env.NOG_LOAD_TEST_CONFIG;
if (config[0] === '{') {
  config = JSON.parse(config);
} else {
  config = JSON.parse(readFileSync(config));
}
console.log('Test config:', config);


loadTestDb = {}

connectLoadTestDb = function(client, done) {
  MongoClient.connect(config.loadTestsMongoUrl, function(err, db) {
    if (err) {
      throw err;
    }
    console.log('Connected to load test db.');
    loadTestDb.connection = db;
    loadTestDb.testUsers = db.collection('testUsers');
    done();
  });
}

closeLoadTestDb = function() {
  loadTestDb.connection.close();
}

saveTestUser = function(uinfo, done) {
  loadTestDb.testUsers.insert(uinfo, function(err, res) {
    if (err) {
      throw(err);
    }
    console.log('Stored test user:', JSON.stringify(uinfo));
    done();
  });
}


// We use prefixes `tusr-` and `pw-` to indicate the context.
//
// The username is timebased with a bit of randomness to reduce the collision
// probability when running concurrent tests.  Colons and dots are avoided, and
// the username is converted to lowercase to avoid potential problems with
// unexpected characters.
//
// The password is purely random.
//
// The email is based on a template string from the config.  We usually use
// email sub-addresses, like `fred+nogtestuser_#{username}@example.com`.

createUserInfo = function() {
  now = new Date().toISOString().replace(/:/g, '').replace(/[.]/g, '-');
  username = 'tusr-' + now + '-' + randomBytes(2).toString('hex');
  username = username.toLowerCase();
  email = config.accountSpec.email.replace('#{username}', username);
  return {
    username: username,
    email: email,
    password: 'pw-' + randomBytes(12).toString('hex')
  };
};


openSignUpForm = function(client) {
  client
    .expect.element('#login-dropdown-list').to.be.visible.before(1000);
  client
    .click('#login-dropdown-list')
    .expect.element('#signup-link').to.be.visible.before(100);
  client
    .click('#signup-link')
    .expect.element('#login-username').to.be.visible.before(100);
  client.expect.element('#login-email').to.be.visible.before(100);
  client.expect.element('#login-password').to.be.visible.before(100);
}


createAccount = function(client, uinfo) {
  client.setValue('#login-username', uinfo.username);
  client.setValue('#login-email', uinfo.email);
  client.setValue('#login-password', uinfo.password);
  client.click('#login-buttons-password');

  // `.text.to.contain(uinfo.username).before(1000)` causes errors `undefined
  // is not a function`.  One hypothesis is that Meteor's way of handling the
  // DOM may confuse Nightwatch: Maybe Meteor creates a new element, and
  // Nightwatch tries to access a stale reference.  Use a short `pause()`
  // instead.

  client.pause(1000);
  client
    .expect.element('#login-dropdown-list > a.dropdown-toggle')
    .text.to.contain(uinfo.username); // `.before(1000)` does not work here.

  return uinfo;
}


// Chrome is configured in `nightwatch.js` to automatically download.
//
// XXX It is unclear how we can configure Firefox to download files
// automatically.  One option might be to create a profile; see
// <https://github.com/nightwatchjs/nightwatch/wiki/Enable-Firebug-in-Firefox-for-Nightwatch-tests>.
//
// We use only Chrome for now.

createAPIKey = function(client, uinfo) {
  apikeyPath = config.downloadDirectory + '/' + 'apikey.sh.txt'
  client
    .click('.navbar-nav a[href*=settings]')
    .expect.element('.js-create-apikey').to.be.visible.before(1000);
  client
    .perform(function() {
      try {
        unlinkSync(apikeyPath);
      } catch(err) {
        // Ignore missing files.
      }
    })
    .click('.js-create-apikey')
    .expect.element('.js-delete-apikey-start').to.be.visible.before(1000);
  client.pause(1000);
  client.perform(function(client, done) {
    apikey = readFileSync(apikeyPath, {encoding: 'utf8'});
    username = apikey.match(/NOG_USERNAME=(\S+)/)[1]
    keyid = apikey.match(/NOG_KEYID=(\S+)/)[1]
    secretkey = apikey.match(/NOG_SECRETKEY=(\S+)/)[1]
    if (username !== uinfo.username) {
      throw new Error('Username of API key does not match.');
    }
    uinfo.keyid = keyid
    uinfo.secretkey = secretkey
    saveTestUser(uinfo, done);
  });
}


logout = function(client) {
  client
    .click('#login-dropdown-list')
    .expect.element('#login-buttons-logout').to.be.visible.before(1000);
  client
    .click('#login-buttons-logout')
  client.pause(1000);
  client
    .expect.element('#login-dropdown-list a.dropdown-toggle')
    .text.to.contain('Sign in');
}


login = function(client, uinfo) {
  client
    .click('#login-dropdown-list');
  client.pause(1000);
  client
    .expect.element('#login-buttons-password').to.be.visible;
  client.setValue('#login-username-or-email', uinfo.username);
  client.setValue('#login-password', uinfo.password);
  client.click('#login-buttons-password');

  client.pause(1000);
  client
    .expect.element('#login-dropdown-list > a.dropdown-toggle')
    .text.to.contain(uinfo.username);
}

confirmAccount = function(client, username) {
  client
    .click('.navbar-nav a[href*=admin]')
  client.useXpath();
  // See <http://stackoverflow.com/a/3655588> for `contain()`.
  xpath = '' +  // Button `+ users`.
    '//td[text()="' + username + '"]' +
    '/parent::*' +
    '/descendant::button[text()[contains(., "users")]]';
  client
    .expect.element(xpath)
    .to.be.visible.before(1000);

  client
    .click(xpath);
  xpath = '' +  // Roles 'users'
    '//td[text()="' + username + '"]' +
    '/parent::*' +
    '/descendant::td[text()="users"]';
  client
    .expect.element(xpath)
    .to.be.visible.after(1000);

  client.useCss();
}


createUser = function(client) {
  uinfo = createUserInfo();
  console.log('Creating account:', JSON.stringify(uinfo));
  openSignUpForm(client);
  createAccount(client, uinfo);
  logout(client);

  login(client, config.admin);
  confirmAccount(client, uinfo.username);
  logout(client);

  login(client, uinfo);
  createAPIKey(client, uinfo);
  logout(client);
}


module.exports = {
  before: function(client, done) {
    connectLoadTestDb(client, done)
  },

  after: function(client, done) {
    closeLoadTestDb();
  },

  'create test users': function(client) {
    client.url(config.url);
    for (i = 0; i < config.nAccounts; i++) {
      createUser(client);
    }
    client.end();
  }
};
