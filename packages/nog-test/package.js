var client = 'client', server = 'server', both = [client, server];

Package.describe({
  name: 'nog-test',
  version: '0.0.1',
  summary: 'Nog testing infrastructure',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use([
    'check',
    'coffeescript@1.12.1 || 2.0.0',
    'modules',
    'underscore'
  ]);

  // Add a weak dependency to monkey-patch `it.if*` conditional skip.
  api.use('meteortesting:mocha', {weak: true})

  api.export('NogTest');
  api.addFiles('nog-test.coffee', both);
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  // No tests so far.  Since the package is used to write tests, it may be
  // sufficient to observe whether it works as expected during testing.
});
