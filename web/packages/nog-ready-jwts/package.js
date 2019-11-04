Package.describe({
  name: 'nog-ready-jwts',
  version: '0.0.0',
  summary: 'Manage JWTs with predefined scopes.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'ecmascript',
    'modules',
  ]);
  // Nog.
  api.use([
    'nog-jwt-2',
  ]);

  api.mainModule('index-server.js', 'server');
  api.mainModule('index-client.js', 'client');
});

Package.onTest(function(api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'ecmascript',
    'modules',
    'jquery',
  ]);
  // Testing.
  api.use([
    'meteortesting:mocha',
  ]);
  // Package to test.
  api.use([
    'nog-ready-jwts',
  ]);

  // Meteor seems to automatically interpret some file names in a way that
  // causes problems.  Therefore, do not use the following file naming:
  //
  //  - `*.tests.js`, for example `server.tests.js`;
  //  - `tests/*.js`, for example `tests/server.js`.
  //
  api.mainModule('server-tests.js', 'server');
  api.mainModule('client-tests.js', 'client');
});
