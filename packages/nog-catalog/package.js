Package.describe({
  name: 'nog-catalog',
  version: '0.0.1',
  summary: 'Catalogs of entries of other repos',
  git: null,
  documentation: 'README.md'
});

Npm.depends({
  'mustache': '2.3.0',
});

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
    'check',
    'ecmascript',
    'templating@1.3.2',
    'ejson',
    'modules',
    'random',
    'underscore',
    'less@1.0.0 || 2.0.0',
  ]);
  // Other.
  api.use([
    'aslagle:reactive-table@0.8.38',
    'natestrauser:publish-performant-counts@0.1.0',
  ]);
  // Nog.
  api.use([
    'nog-content',
    'nog-error',
    'nog-settings',
    'nog-search',
  ]);
  api.use('nog-access', { weak: true });

  api.addFiles([
    './nog-catalog-ui.less',
  ], 'client');

  api.mainModule('index-server.js', 'server');
  api.mainModule('index-client.js', 'client');
  api.export('NogCatalog');
});

Package.onTest(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
    'ecmascript',
    'modules',
    'mongo',
    'random',
    'underscore',
  ]);

  // Other.
  api.use([
    'johanbrook:publication-collector@1.0.2',
    'meteortesting:mocha',
  ]);

  // Nog.
  api.use([
    'nog-catalog',
    'nog-content',
  ]);

  // Server tests.
  api.addFiles([
    'catalog-pipeline-tests.js',
    'concurrency-tests.js',
    'nog-catalog-server-tests.js',
  ], 'server');

  // Client tests.
  api.addFiles([
    'nog-catalog-client-tests.js',
  ], 'client');
});
