Package.describe({
  name: 'mongo-ssl',
  version: '1.0.0-0',
  summary: 'Configure Mongo SSL options from environment variables',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.4.3.1');
  api.use([
    'ecmascript',
    'modules',
    'mongo',
  ]);

  api.mainModule('index-server.js', 'server');
});
