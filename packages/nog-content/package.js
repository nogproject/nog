var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-content',
  version: '0.0.1',
  summary: 'Nog git-like content store.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  // Core:
  api.use([
    'accounts-base',
    'check',
    'coffeescript@1.12.1 || 2.0.0',
    'ecmascript',
    'ejson',
    'check',
    'modules',
    'mongo',
    'random',
    'underscore',
  ]);
  // Other:
  api.use([
    'jparker:crypto-core@0.1.0',
    'jparker:crypto-sha1@0.1.0',
    'momentjs:moment@2.19.1',
  ]);
  // Nog:
  api.use([
    'nog-error',
    'nog-settings',
  ]);
  // Nog weak:
  api.use([
    'nog-access',
    'nog-blob',
  ], {weak: true});

  api.mainModule('index-client.js', client);
  api.mainModule('index-server.js', server);
  api.export('NogContent');
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  // Core:
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'ejson',
    'modules',
    'mongo',
    'random',
    'underscore',
  ]);
  // Other:
  api.use([
    'jparker:crypto-sha1@0.1.0',
    'momentjs:moment@2.19.1',
    'meteortesting:mocha',
  ]);
  // Nog:
  api.use([
    'nog-error',
    'nog-test',
  ]);
  // Package under test:
  api.use('nog-content');

  // The order matters.
  api.addFiles('idversion-testdata.js', server);
  api.addFiles('nog-content-caching-server-tests.coffee', server);
  api.addFiles('nog-content-server-tests.coffee', server);
});
