Package.describe({
  name: 'nog-jwt-2',
  version: '0.0.0',
  summary: 'Nog JWT auth',
  git: null,
  documentation: 'README.md',
});

// See package-peer-versions.js for Npm peer dependencies that must be
// installed in the application.

Package.onUse(function (api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'check',
    'ecmascript',
    'modules',
  ]);
  // Other.
  api.use([
    'tmeasday:check-npm-versions@0.3.1',
  ]);
  // Nog.
  api.use([
    'nog-error-2',
    'nog-error-codes',
  ]);
  api.mainModule('index-server.js', 'server');
});

// No package tests.  See `fso-testapp/tests` for tests.
