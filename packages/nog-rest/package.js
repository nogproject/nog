var client = 'client', server = 'server', both = [client, server];

Package.describe({
  name: 'nog-rest',
  version: '0.0.1',
  summary: 'REST Api infrastructure.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'underscore',
    'check',
  ]);
  api.use('modules');
  api.use('meteorhacks:picker@1.0.2');
  api.use('nog-error');
  api.use('nog-auth', {weak: true});

  api.export('NogRest');

  api.addFiles('nog-rest.coffee', server);
});

Npm.depends({
  "path-to-regexp": "1.0.3",
  "body-parser": "1.12.3"
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use('modules');
  api.use(['meteortesting:mocha']);
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'underscore',
  ]);
  api.use('http');
  api.use(['check', 'random']);
  api.use('nog-test');
  api.use('nog-error');
  api.use('nog-rest');

  api.addFiles('nog-rest-tests.coffee', server);
});
