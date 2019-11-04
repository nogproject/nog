Package.describe({
  name: 'nog-fso-authz',
  version: '0.0.0',
  summary: 'Managing Nog access statements for FSO in Nog App 2',
  git: null,
  documentation: 'README.md'
});

// See package-peer-versions.js for Npm peer dependencies that must be
// installed in the application.

Package.onUse(function(api) {
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

Package.onTest(function(api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'ecmascript',
    'jquery',
    'modules',
    'mongo',
    'http',
  ]);
  // Testing.
  api.use([
    'meteortesting:mocha',
  ]);
  // Nog.
  api.use([
    'nog-access-2',
  ]);
  // Package to test.
  api.use([
    'nog-fso-authz',
  ]);

  // Meteor seems to automatically interpret some file names in a way that
  // causes problems.  Therefore, do not use the following file naming:
  //
  //  - `*.tests.js`, for example `server.tests.js`;
  //  - `tests/*.js`, for example `tests/server.js`.
  //
  api.mainModule('server-tests.js', 'server');
});
