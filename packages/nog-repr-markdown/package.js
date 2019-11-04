var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-repr-markdown',
  version: '0.0.1',
  summary: 'Plugin for markdown entries.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
    'ecmascript',
    'check',
    'coffeescript@1.12.1 || 2.0.0',
    'modules',
    'ejson',
    'less@1.0.0 || 2.0.0',
    'reactive-dict',
    'session',
    'templating@1.3.2',
    'underscore'
  ]);

  // other packages.
  api.use([
    'mquandalle:jade@0.4.1',
    'chuangbo:marked@0.3.5',
    'tmeasday:check-npm-versions@0.3.1',
  ]);

  // nog packages.
  api.use([
    'nog-error',
    'nog-content',
    'nog-access',
    'nog-blob',
    'markdown-toc@0.11.5'
  ]);
  api.use('nog-files', {weak: true});

  api.addFiles([
    'package-peer-versions.js',
  ], server);

  api.addFiles([
    'nog-repr-markdown.coffee'
  ], anywhere);
  api.addFiles([
    'nog-repr-markdown-ui.less',
    'nog-repr-markdown-ui.jade',
    'nog-repr-markdown-ui.coffee'
  ], client);
});
