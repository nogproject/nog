Package.describe({
  name: 'nog-error-codes',
  version: '0.0.0',
  summary: 'Nog error specs that are used by multiple packages',
  git: null,
  documentation: 'README.md',
});

Package.onUse(function (api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'ecmascript',
    'modules',
  ]);

  api.mainModule('index-server.js', 'server');
  api.mainModule('index-client.js', 'client');
});

// No package tests.
