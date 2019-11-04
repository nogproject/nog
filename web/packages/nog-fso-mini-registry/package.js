Package.describe({
  name: 'nog-fso-mini-registry',
  version: '0.0.0',
  summary: 'A simple Nogfsoregd read model to resolve repo IDs and paths',
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
  // Nog.
  api.use([
    'nog-error-2',
  ]);
  api.mainModule('index-server.js', 'server');
});

// No package tests.  See `fso-testapp/tests` for tests.
