var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-fmt',
  version: '0.0.1',
  summary: 'Formatting functions for Nog.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
    'check',
    'coffeescript@1.12.1 || 2.0.0',
    'modules',
    'underscore'
  ]);

  api.export('NogFmt');

  api.addFiles([
    'nog-fmt.coffee',
    'mustache.coffee'
  ], anywhere);
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use('modules');
  api.use('meteortesting:mocha');
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'underscore',
  ]);
  api.use('nog-fmt');

  api.addFiles([
    'nog-fmt-tests.coffee'
  ], anywhere);
});
