Package.describe({
  name: 'nog-fso-ui',
  version: '0.0.1',
  summary: 'Nog file system observer UI',
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
    'promise',
    'underscore',
    'react-meteor-data',
  ]);
  // Other.
  api.use([
    'tmeasday:check-npm-versions@0.3.1',
    'gadicc:blaze-react-component@1.4.0',
  ]);
  // Nog.
  api.use([
    'nog-error',
  ]);

  api.mainModule('index-server.js', 'server');
  api.mainModule('index-client.js', 'client');
});
