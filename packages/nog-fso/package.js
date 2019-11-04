Package.describe({
  name: 'nog-fso',
  version: '0.0.1',
  summary: 'Nog file system observer',
  git: null,
  documentation: 'README.md'
});

// See package-peer-versions.js for Npm peer dependencies that must be
// installed in the application.

Package.onUse(function(api) {
  // Core.
  api.versionsFrom('1.6.0.1');
  api.use([
    'check',
    'ecmascript',
    'modules',
    'promise',
    'underscore',
  ]);
  // Other.
  api.use([
    'tmeasday:check-npm-versions@0.3.1',
  ]);
  // Nog.
  api.use([
    'nog-error',
  ]);

  api.addAssets([
    'proto/nogfsopb/broadcast.proto',
    'proto/nogfsopb/discovery.proto',
    'proto/nogfsopb/git-details.proto',
    'proto/nogfsopb/gitnog.proto',
    'proto/nogfsopb/gitnogtree.proto',
    'proto/nogfsopb/job-control.proto',
    'proto/nogfsopb/registry.proto',
    'proto/nogfsopb/repo-init.proto',
    'proto/nogfsopb/repos.proto',
    'proto/nogfsopb/root-info.proto',
    'proto/nogfsopb/stat.proto',
    'proto/nogfsopb/tartt.proto',
    'proto/nogfsopb/workflows.proto',
  ], 'server');

  api.mainModule('index-server.js', 'server');
  api.mainModule('index-client.js', 'client');
});
