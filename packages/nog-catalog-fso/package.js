Package.describe({
  name: 'nog-catalog-fso',
  version: '0.0.1',
  summary: 'Nog catalog file system observer plugin',
  git: null,
  documentation: 'README.md'
});

// See package-peer-versions.js for Npm peer dependencies that must be
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
    'tmeasday:check-npm-versions@0.3.1',
  ]);
  // Nog.
  api.use([
    'nog-error',
  ]);

  api.mainModule('index-server.js', 'server');
});
