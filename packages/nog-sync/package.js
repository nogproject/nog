var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-sync',
  version: '0.0.1',
  summary: 'Synchronize Nog deployments.',
  git: null,
  documentation: 'README.md'
});

Npm.depends({
  'priorityqueuejs': '1.0.0'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // Core Meteor.
  api.use([
    'accounts-base',
    'check',
    'coffeescript',
    'ddp',
    'ecmascript',
    'ejson',
    'modules',
    'mongo',
    'random',
    'underscore',
  ]);

  // Other packages.
  api.use([
    'alanning:roles@1.2.13',
    'jparker:crypto-sha1@0.1.0',
    'momentjs:moment@2.10.3'
  ]);

  // Nog packages.
  api.use([
    'nog-content',
    'nog-error',
    'nog-auth'
  ]);

  api.mainModule('index.coffee', server);
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  // Test driver.
  api.use([
      'practicalmeteor:mocha',
      'practicalmeteor:chai',
  ]);

  // Core Meteor.
  api.use([
    'coffeescript',
    'ecmascript',
    'ejson',
    'modules',
    'mongo',
    'random',
    'underscore',
  ]);

  // Other packages.
  api.use([
    'jparker:crypto-sha1@0.1.0',
    'momentjs:moment@2.10.3',
  ]);

  api.use([
    'nog-content',
    'nog-error',
    'nog-sync',
    'nog-test'
  ]);

  api.addFiles(
    [
      'nog-sync-store-tests.coffee',
      'nog-sync-diff-tests.coffee',
      'nog-sync-peer-tests.coffee',
      'nog-sync-mergebases-tests.js',
      'nog-sync-merge-tests.coffee',
      'nog-sync-apply-tests.js',
      'nog-sync-remote-basic-tests.coffee',
      'nog-sync-remote-tests.coffee',
      'nog-sync-peer-ping-tests.coffee',
      'nog-sync-peer-sync-tests.coffee'
    ],
    server
  );
});
