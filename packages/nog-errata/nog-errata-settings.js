import { defSetting } from 'meteor/nog-settings';


defSetting({
  key: 'public.errata',
  val: [],
  help: `
\`errata\` is a list of errata descriptions \`{code, description}\`.  See
package \`nog-errata\` for details.
`,
  match: [{ code: String, description: String }],
});
