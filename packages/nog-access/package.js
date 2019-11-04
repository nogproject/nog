var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-access',
  version: '0.0.1',
  summary: 'Nog access checks',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use('coffeescript@1.12.1 || 2.0.0');
  api.use('modules');
  api.use('ecmascript');
  api.use(['check', 'underscore', 'ejson']);
  api.use(['accounts-base', 'alanning:roles@1.2.13']);
  api.use('reactive-dict');
  api.use('nog-error');
  api.use('nog-settings');
  api.use('mongo');
  api.use('templating@1.3.2');

  api.export('NogAccess', anywhere);
  api.export('NogAccessTest', server, {testOnly: true});

  api.addFiles('nog-access.coffee', anywhere);
  api.addFiles('nog-access-server.coffee', server);
  api.addFiles('nog-access-statements.coffee', server);
  api.addFiles('nog-access-client.coffee', client);

});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use('modules');
  api.use(['meteortesting:mocha']);
  api.use(['coffeescript@1.12.1 || 2.0.0', 'underscore']);
  api.use(['accounts-password', 'alanning:roles@1.2.13']);
  api.use(['test-helpers', 'random', 'jquery']);
  api.use('nog-test');
  api.use('nog-access');

  api.addFiles('nog-access-server-tests.coffee', server);
});
