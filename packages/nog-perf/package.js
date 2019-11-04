var client = 'client', server = 'server', anywhere = [client, server];

Package.describe({
  name: 'nog-perf',
  version: '0.0.1',
  summary: 'Performance tuning tools: a wrapper around v8-profiler.',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.versionsFrom('1.6.0.1');
  api.use([
    'coffeescript@1.12.1 || 2.0.0',
    'underscore',
  ]);
  api.use('modules');

  api.export('NogPerf', anywhere);
  api.addFiles('nog-perf.coffee', anywhere);
  api.addFiles('nog-profiler-server.coffee', server);
  api.addFiles('nog-profiler-client.coffee', client);
});

Npm.depends({
  'v8-profiler': '5.6.5'
});
