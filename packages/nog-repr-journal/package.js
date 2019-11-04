Package.describe({
  name: 'nog-repr-journal',
  version: '0.0.1',
  summary: 'Displays journals.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
      'check',
      'ecmascript',
      'modules',
      'ejson',
      'templating@1.3.2'
    ]);


  // nog packages.
  api.use([
      'nog-content',
      'nog-files',
      'nog-repr-markdown'
    ]);

  api.mainModule('index-client.js', 'client');
  api.mainModule('index-server.js', 'server');
});
