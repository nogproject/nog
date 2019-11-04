Package.describe({
  name: 'nog-cluster',
  version: '0.0.1',
  summary: 'Server code to manage a cluster of application servers.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  // Core:
  api.versionsFrom('1.6.0.1');
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'ecmascript',
    'modules',
    'mongo',
    'random',
    'underscore'
  ]);
  // Nog:
  api.use([
    'nog-settings',
  ]);

  api.mainModule('index-server.js', 'server');
  api.export('NogCluster', 'server');
});
