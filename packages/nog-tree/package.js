var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-tree',
  version: '0.0.1',
  summary: 'Nog tree browsing UI.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
    'accounts-base',
    'check',
    'coffeescript@1.12.1 || 2.0.0',
    'ecmascript',
    'modules',
    'ejson',
    'less@1.0.0 || 2.0.0',
    'mongo',
    'reactive-dict',
    'session',
    'templating@1.3.2',
    'underscore'
  ]);

  // other packages.
  api.use([
    'mquandalle:jade@0.4.1',
    'ccorcos:subs-cache@0.9.3',
  ]);

  // nog packages.
  api.use([
    'nog-access',
    'nog-blob',
    'nog-content',
    'nog-error',
    'nog-fmt',
    'nog-repo-toolbar',
    'nog-settings',
    'nog-widget',
  ]);

  api.export('NogTree');

  api.addFiles('nog-tree.coffee', anywhere);
  api.addFiles('nog-tree-server.coffee', server);
  api.addFiles([
    'nog-tree-ui.less',
    'nog-tree-ui.jade',
    'nog-tree-ui.coffee'
  ], client);
});
