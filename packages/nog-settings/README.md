# Package `nog-settings`

`nog-setting` helps managing Meteor settings.

## `defSettings({ key, val, help, match })` (server)

`defSettings()` helps managing Meteor settings.  `key` is a dot path in
`Meteor.settings`.  `val` is the default value that will be used if none is
provided by the environment.  `help` is a short help text, starting with the
last part of `<key>` by convention.  `match` is a Meteor check match pattern to
validate the settings value.

The recommendation is to use `defSettings()` in a separate file
`nog-<package>-settings.js` to define all settings early during package
initialization.

Example `nog-<package>-settings.js`:

```js
import { defSettings } from 'meteor/nog-settings';

defSetting({
  key: 'public.upload.concurrentUploads',
  val: 3,
  help: `
\`concurrentUploads\` limits the number of concurrent file uploads from
a browser.
`,
  match: matchPositiveNumber,
});
```
