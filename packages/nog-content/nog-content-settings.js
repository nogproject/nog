import { defSetting } from 'meteor/nog-settings';


defSetting({
  key: 'optStrictRepoMembership',
  val: true,
  help: `
\`optStrictRepoMembership=true\` enables checks that ensure that repo content
can be only accessed if it is reachable from a ref or if it has been recently
added to the repo.  See package \`nog-content\` devdoc README for details.
`,
  match: Boolean,
});
