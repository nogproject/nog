Package.describe({
  name: 'markdown-toc',
  version: '0.11.5_2',
  summary: 'Wrap npm package `markdown-toc` for Meteor',
  git: null,
  documentation: 'README.md'
});

Package.onUse(function(api) {
  api.use([
      'modules',
  ]);

  api.addFiles('index-client.js', 'client');

  api.export('MarkdownToc', 'client');
});

Npm.depends({
  'markdown-toc': '0.11.5'
});
