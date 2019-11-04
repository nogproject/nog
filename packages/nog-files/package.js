var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-files',
  version: '0.0.1',
  summary: 'Nog file browsing UI.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
    'accounts-base',
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
    'check@1.1.0',
    'mquandalle:jade@0.4.1',
    'peppelg:bootstrap-3-modal@1.0.3'
  ]);

  // nog packages.
  api.use([
    'nog-blob',
    'nog-content',
    'nog-error',
    'nog-fmt',
    'nog-repo-toolbar',
    'nog-settings',
    'nog-widget',
  ]);
  api.use('nog-access', {weak:true});
  api.use('nog-repo-toolbar', {weak:true});
  api.use('nog-errata', {weak:true});

  api.addFiles('nog-files.coffee', anywhere);
  api.addFiles([
    'nog-files-ui.less',
    'nog-files-ui.html',
    'nog-files-ui.jade',
    'nog-files-ui.coffee',
    'nog-files-ui.js',
  ], client);

  // XXX Minimal module exports to work with Meteor 1.3.3.  `share` is used to
  // pass API objects internally between `.coffee` files; the export uses
  // `mainModule()`.  Without this, `api.export()` misses exported objects for
  // unknown reasons.  They way forward is to restructure the code to replace
  // `addFiles()` by internal use of require / import.

  api.mainModule('index.coffee', anywhere);
  api.export('NogFiles');
});
