var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-widget',
  version: '0.0.1',
  summary: 'Widgets for Nog.',
  git: null,
  documentation: 'README.md',
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');

  // core Meteor.
  api.use([
      'coffeescript@1.12.1 || 2.0.0',
      'templating@1.3.2',
      'modules',
      'ecmascript',
      'less@1.0.0 || 2.0.0'
  ]);

  // other packages.
  api.use([
      'mquandalle:jade@0.4.1',
      'aldeed:simple-schema@1.5.3',
      'twbs:bootstrap',
  ]);

  // nog packages.
  api.use([
    'nog-error',
    'nog-settings',
  ]);

  api.addFiles('nog-widget-server.coffee', server);
  api.addFiles('nog-widget.coffee', anywhere);
  api.addFiles([
    'nog-widget-ui.less',
    'nog-widget-ui.jade',
    'nog-widget-ui.coffee',
    'nog-widget-ui.js',
  ], client);
});
