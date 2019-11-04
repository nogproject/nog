var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-flow',
  version: '0.0.1',
  summary: 'Nog workflow UI',
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
    'less@1.0.0 || 2.0.0',
    'mongo',
    'reactive-dict',
    'templating@1.3.2',
    'underscore'
  ]);

  // other packages.
  api.use([
    'mquandalle:jade@0.4.1'
  ]);

  // nog packages.
  api.use([
    'nog-access',
    'nog-blob',
    'nog-cluster',
    'nog-content',
    'nog-error',
    'nog-fmt',
    'nog-repo-toolbar',
    'nog-widget',
  ]);
  api.use('nog-errata', {weak:true});

  // File order matters.
  api.addFiles('nog-flow.coffee', anywhere);
  api.addFiles([
    'nog-flow-ui.less',
    'nog-flow-ui.jade',
    'nog-flow-ui.coffee'
  ], client);

  // The package has not been fully ported to modules.  `share` is still used
  // to pass API objects internally between `.coffee` files.  `share` is then
  // exported via `mainModule()`.  Some files are directly imported in index
  // files and do not appear in `addFiles()`.  The way forward is to replace
  // more `addFiles()` by internal use of import.

  api.mainModule('index-server.coffee', server);
  api.mainModule('index-client.coffee', client);
  api.export('NogFlow');
});
