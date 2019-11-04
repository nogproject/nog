Package.describe({
  name: 'nog-search',
  version: '0.0.1',
  summary: 'Nog search UI.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use([
    'check',
    'ecmascript',
    'templating@1.3.2',
    'modules',
    'coffeescript@1.12.1 || 2.0.0',
    'less@1.0.0 || 2.0.0',
    'mongo',
    'reactive-dict',
    'underscore',
  ]);
  api.use([
    'mquandalle:jade@0.4.5',
    'easy:search@2.0.9',
    'aldeed:simple-schema@1.5.3',
    'mdg:validated-method@1.1.0',
  ]);
  api.use([
    'nog-error',
    'nog-content',
    'nog-cluster',
  ]);

  api.addFiles([
    './nog-search-ui.less',
  ], 'client');

  api.mainModule('index-client.js', 'client');
  api.mainModule('index-server.js', 'server');
});

Package.onTest(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
    'ecmascript',
    'modules',
    'blaze',
    'templating@1.3.2',
  ]);

  // Other.
  api.use([
    'meteortesting:mocha',
  ]);

  // Nog.
  api.use([
    'nog-search',
  ]);

  // Client tests.
  api.addFiles([
    'nog-search-input-form-tests.js',
  ], 'client');
});
