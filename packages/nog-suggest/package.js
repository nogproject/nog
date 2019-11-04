Package.describe({
  name: 'nog-suggest',
  version: '0.0.1',
  summary: 'Nog auto suggest',
  git: null,
  documentation: 'README.md'
});

// See package-peer-versions.js for NPM peer dependencies that must be
// installed in the application.

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
    'check',
    'ecmascript',
    'modules',
    'underscore',
  ]);
  // Other.
  api.use([
    'tmeasday:check-npm-versions@0.3.2',
  ]);
  // Nog.
  api.use([
    'nog-error',
  ]);

  api.mainModule('index-server.js', 'server');
  api.mainModule('index-client.js', 'client');
});
