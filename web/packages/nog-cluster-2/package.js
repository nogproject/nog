Package.describe({
  name: 'nog-cluster-2',
  version: '0.0.0',
  summary: 'Nog example package',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'ecmascript',
    'modules',
    'mongo',
    'random',
  ]);
  // Other.
  api.use([
    'tmeasday:check-npm-versions@0.3.1',
  ]);

  api.mainModule('index-server.js', 'server');
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
    'nog-cluster-2',
  ]);

  // Meteor seems to automatically interpret some file names in a way that
  // causes problems.  Therefore, do not use the following file naming:
  //
  //  - `*.tests.js`, for example `server.tests.js`;
  //  - `tests/*.js`, for example `tests/server.js`.
  //
  api.mainModule('server-tests.js', 'server');
});
