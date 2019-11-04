Package.describe({
  name: 'nog-ldap',
  version: '0.0.1',
  summary: 'Handling connections to LDAP servers for user info',
  git: null,
  documentation: 'README.md'
});

// Required Meteor Npm peer dependencies in application:
//
// - ldapjs@~1.0.1

Package.onUse(function(api) {
  // Core:
  api.versionsFrom('1.6.0.1');
  api.use([
    'ecmascript',
  ]);
  // Other.
  api.use([
    'tmeasday:check-npm-versions@0.3.1',
  ]);

  api.mainModule('index-server.js', 'server');
});
