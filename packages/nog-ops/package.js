var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-ops',
  version: '0.0.1',
  summary: 'Nog operations tooling',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // Core Meteor.
  api.use([
    'check',
    'ecmascript',
    'ejson',
    'modules',
    'random',
    'underscore',
  ]);

  // Other packages.
  api.use([
  ]);

  // Nog packages.
  api.use([
  ]);
  api.use('nog-access', { weak: true });

  api.mainModule('index-client.js', client);
  api.mainModule('index-server.js', server);
  api.export('NogOps');
});
