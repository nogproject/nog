var client = 'client', server = 'server', both = [client, server];

Package.describe({
  name: 'nog-auth',
  version: '0.0.1',
  summary: 'signature-based authentication',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'underscore',
  ]);
  api.use('modules');
  api.use(['templating@1.3.2', 'mquandalle:jade@0.4.1']);
  api.use('accounts-base');
  api.use('check');
  api.use('reactive-var');
  api.use('nog-error');
  api.use('nog-access', {weak: true});

  api.addFiles('nog-auth-ui.jade', client);
  api.addFiles('nog-auth.coffee', both);
  api.addFiles('nog-auth-ui.coffee', client);
  api.addFiles('nog-auth-server.coffee', server);

  // XXX Minimal module exports to work with Meteor 1.3.3.  `share` is used to
  // pass API objects internally between `.coffee` files; the export uses
  // `mainModule()`.  Without this, `api.export()` misses exported objects for
  // unknown reasons.  They way forward is to restructure the code to replace
  // `addFiles()` by internal use of require / import.

  api.mainModule('nog-auth-index.coffee', both);
  api.export('NogAuth');
  api.export('NogAuthTest', {testOnly: true});
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use('modules');
  api.use(['meteortesting:mocha']);
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'underscore',
    'mongo',
    'templating@1.3.2',
    'jquery',
  ]);
  api.use('http');
  api.use('test-helpers');
  api.use(['accounts-password', 'random']);
  api.use('check');
  api.use('nog-test');
  api.use('nog-error');
  api.use('nog-auth');

  api.addFiles('nog-auth-server-tests.coffee', server);
});
