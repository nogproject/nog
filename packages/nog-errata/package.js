Package.describe({
  name: 'nog-errata',
  version: '0.0.1',
  summary: 'Handling of specific data correctness issues.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  // Core:
  api.versionsFrom('1.6.0.1');
  api.use([
    'ecmascript',
    'modules',
    'templating@1.3.2',
  ]);
  // Nog:
  api.use([
    'nog-settings',
  ]);

  api.mainModule('index-client.js', 'client');
  api.mainModule('index-server.js', 'server');
});
