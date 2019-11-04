Package.describe({
  name: 'nog-settings',
  version: '0.0.1',
  summary: 'Managing Meteor settings',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
    'check',
    'ecmascript',
    'modules',
    'underscore',
  ]);

  api.mainModule('index-server.js', 'server');
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  // Core.
  api.use([
      'check',
      'ecmascript',
      'modules',
      'underscore',
  ]);
  // Other.
  api.use([
      'meteortesting:mocha',
  ]);
  // Nog.
  api.use([
      'nog-settings',
      'nog-test',
  ]);

  api.addFiles('nog-settings-tests.js', 'server');
});
