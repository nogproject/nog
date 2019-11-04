var client = 'client', server = 'server', both = [client, server];

Package.describe({
  name: 'nog-blob',
  version: '0.0.1',
  summary: 'Blob upload and download using S3 object storage',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
      'accounts-base',
      'check',
      'coffeescript@1.12.1 || 2.0.0',
      'ecmascript',
      'modules',
      'mongo',
      'templating@1.3.2',
      'tracker',
      'underscore',
  ]);
  // Other.
  api.use([
      'mquandalle:jade@0.4.1',
  ]);
  // Nog.
  api.use([
      'nog-error',
      'nog-multi-bucket',
      'nog-settings',
  ]);
  api.use('nog-access', {weak: true});

  api.addFiles(['uploadHeading.tpl.jade', 'uploadItem.tpl.jade'], client);
  api.addFiles('aBlobHref.tpl.jade', client);

  api.addFiles('nog-blob-settings.js', server);
  api.addFiles('nog-blob.coffee', both);
  api.addFiles('nog-blob-client.coffee', client);
  api.addFiles('nog-blob-bucket-router.js', server);
  api.addFiles('nog-blob-server.coffee', server);
  api.addFiles('nog-blob-migrations.coffee', server);

  // The URL for static files is `/packages/<name>/<path>`.  The sha1 is
  // mangled into the filename to ensure that the correct code is used with any
  // cache settings.
  //
  // The sha paths need to be updated here and in the Hasher implementations.
  //
  // To update Rusha, manually compute the SHA1 and update the symlink.
  //
  // To update Spark MD5, run `npm run minify-spark`.

  api.addAssets([
    'js/rusha-b601dbae5b34a4a08fbf7cc7252a940443a45cde.js',
    'js/spark-md5.min.da8469403d5f743dd3cb0762146f7b6b67f38867.js',
  ], client);

  // XXX Minimal module exports to work with Meteor 1.3.3.  `share` is used to
  // pass API objects internally between `.coffee` files; the export uses
  // `mainModule()`.  Without this, `api.export()` misses exported objects for
  // unknown reasons.  We should eventually restructure the code to replace
  // `addFiles()` by internal use of require / import.

  api.mainModule('index.coffee');
  api.export('NogBlob');
  api.export('NogBlobTest', both, {testOnly: true});
});

Package.onTest(function(api) {
  api.versionsFrom('1.6.0.1');
  // Core.
  api.use([
    'check',
    'coffeescript@1.12.1 || 2.0.0',
    'ecmascript',
    'modules',
    'mongo',
    'random',
    'underscore',
  ]);
  // Other.
  api.use(['meteortesting:mocha']);
  // Nog.
  api.use([
      'nog-blob',
      'nog-test',
  ]);

  api.addFiles('nog-blob-server-tests.coffee', server);
  api.addFiles('nog-blob-client-tests.js', client);
});
