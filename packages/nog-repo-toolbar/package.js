Package.describe({
  name: 'nog-repo-toolbar',
  version: '0.0.1',
  summary: 'Top bar for repository views in Nog.',
  git: null,
  documentation: 'README.md',
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'ecmascript',
    'templating@1.3.2',
    'reactive-var',
    'check',
  ]);

  // other packages.
  api.use([
    'mquandalle:jade@0.4.1',
  ]);

  // nog packages.
  api.use([
    'nog-content',
    'nog-error',
    'nog-widget',
    'nog-access',
  ]);

  api.mainModule('index-client.js', 'client');
  api.mainModule('index-server.js', 'server');
});


Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
    'ecmascript',
    'modules',
    'jquery',
    'blaze',
    'templating@1.3.2',
  ]);

  // other packages.
  api.use([
    'meteortesting:mocha',
  ]);

  // nog packages.
  api.use([
    'nog-repo-toolbar',
  ]);

  // client tests.
  api.addFiles([
    'nog-repo-toolbar-tests.js',
  ], 'client');
});
