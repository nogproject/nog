var client = 'client', server = 'server', both = [client, server];

Package.describe({
  name: 'nog-error',
  version: '0.0.1',
  summary: 'Nog error helper functions and error codes',
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
  api.use([
    'templating@1.3.2',
  ]);
  api.use('check');
  api.use('session');
  api.use('random');
  api.use('mongo');
  api.use('modules');

  api.export('NogError');

  api.addFiles('errorDisplay.html', client);
  api.addFiles('errorDisplay.coffee', client);
  api.addFiles('nog-error.coffee', both);
  api.addFiles('nog-error-specs.coffee', both);
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use('modules');
  api.use(['meteortesting:mocha']);
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'templating@1.3.2',
    'underscore',
    'session',
    'jquery',
  ]);
  api.use('test-helpers');
  api.use('nog-error');

  api.addFiles('nog-error-tests.coffee');
});
