var client = 'client', server = 'server', both = [client, server];

Package.describe({
  name: 'nog-s3',
  version: '0.0.1',
  summary: 'A small wrapper around AWS that exposes just enough of S3 to implement nog-blob',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use(['coffeescript', 'check', 'underscore']);
  api.use('modules');
  api.use('nog-error');

  api.export('S3', server);
  api.addFiles('nog-s3.coffee', server);
});

Npm.depends({

  // Use `aws-sdk` directly (not through the Meteor package
  // `peerlibrary:aws-sdk`) to be able to apply the workaround for package
  // tests; see `Npm.require()` call in source.

  'aws-sdk': '2.2.42',

  // The tests use 'request' to PUT data to S3.

  request: '2.55.0',

});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use(['practicalmeteor:mocha']);
  api.use(['practicalmeteor:chai']);
  api.use('underscore');
  api.use('coffeescript');
  api.use('nog-test');
  api.use('nog-s3');

  api.addFiles('nog-s3-tests.coffee', server);
});
