Package.describe({
  name: 'nog-accounts',
  version: '0.0.0',
  summary: 'GitLab OIDC accounts for Nog App 2',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.8.0.2');
  api.use([
    'check',
    'ecmascript',
    'http',
    'modules',
    'oauth',
    'oauth-encryption',
    'random',
  ]);
  // Nog.
  api.use([
    'nog-error-2',
    'nog-ldap',
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
    'nog-accounts',
  ]);

  // Meteor seems to automatically interpret some file names in a way that
  // causes problems.  Therefore, do not use the following file naming:
  //
  //  - `*.tests.js`, for example `server.tests.js`;
  //  - `tests/*.js`, for example `tests/server.js`.
  //
  api.mainModule('server-tests.js', 'server');
});
