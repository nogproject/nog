var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-repr-example',
  version: '0.0.1',
  summary: 'Example entry representations for nog-tree.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
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
      'mquandalle:jade@0.4.1'
    ]);

  // nog packages.
  api.use([
      'nog-error',
      'nog-content',
      'nog-access',
      'nog-blob',
    ]);
  api.use([
      'nog-tree',
      'nog-files'
    ], {weak: true});

  api.addFiles([
      'nog-repr-example-server.coffee'
    ], server);
  api.addFiles([
      'nog-repr-example-ui.less',
      'nog-repr-example-ui.jade',
      'nog-repr-example-ui.coffee'
    ], client);
});
