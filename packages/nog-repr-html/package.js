var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-repr-html',
  version: '0.0.1',
  summary: 'Plugin for html entries.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
    'ecmascript',
    'coffeescript@1.12.1 || 2.0.0',
    'modules',
    'less@1.0.0 || 2.0.0',
    'templating@1.3.2',
    'underscore'
  ]);

  // other packages.
  api.use([
    'mquandalle:jade@0.4.1',
    'tmeasday:check-npm-versions@0.3.1',
  ]);

  // nog packages.
  api.use([
    'nog-content',
    'nog-blob',
  ]);

  api.addFiles([
    'package-peer-versions.js',
  ], server);

  api.addFiles([
    'nog-repr-html.coffee'
  ], anywhere);
  api.addFiles([
    'nog-repr-html-ui.less',
    'nog-repr-html-ui.jade',
    'nog-repr-html-ui.coffee'
  ], client);
});
