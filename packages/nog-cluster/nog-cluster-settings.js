import { defSetting } from 'meteor/nog-settings';


defSetting({
  key: 'cluster.optSingleInstanceMode',
  val: true,
  help: `
\`optSingleInstanceMode=true\` configures an app instance to immediately take
responsibility for all background tasks.  It is only recommended with a single
app instance for local testing.
`,
  match: Boolean,
});
