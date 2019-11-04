Package.describe({
  name: 'nog-multi-bucket',
  version: '0.0.1',
  summary: 'Support for multiple Nog blob buckets',
  git: null,
  documentation: 'README.md'
});

// Required Meteor Npm peer dependencies in application:
//
// - aws-sdk@~2.5.4

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
    'check',
    'ecmascript',
    'ejson',
    'modules',
    'random',
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
