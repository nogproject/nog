Package.describe({
  name: 'nog-example-2',
  version: '0.0.0',
  summary: 'Nog example package',
  git: null,
  documentation: 'README.md'
});

// See package-peer-versions.js for Npm peer dependencies that must be
// installed in the application.

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'ecmascript',
    'modules',
  ]);
  // Other.
  api.use([
    'tmeasday:check-npm-versions@0.3.1',
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
    'nog-example-2',
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
