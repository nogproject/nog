# Package `nog-settings-2`

## Introduction

`nog-setting-2` helps handling Meteor settings.

## `nog-settings-2.createSettingsParser()` (server)

Create a settings parser.  Example:

```javascript
import { createSettingsParser } from 'meteor/nog-settings';

parser = createSettingsParser();
parser.defSetting({
  key: 'public.upload.concurrentUploads',
  val: 3,
  help: `
\`concurrentUploads\` limits the number of concurrent file uploads from
a browser.
`,
  match: matchPositiveNumber,
});
parser.parseSettings(Meteor.settings);
```

## `parser.defSettings()` (server)

`defSettings({ key, val, help, match })` defines a setting that will be used by
`parser.parseSettings(settings)`.  `key` is a dot path in `settings`.  `val` is
the default value that will be used if none is provided in `settings`.  `help`
is a short help text, starting with the last part of `<key>` by convention.
`match` is a Meteor check match pattern to validate the settings value.

## `parser.parseSettings()` (server)

`parseSettings(settings)` parses `settings` by applying the rules that were
previously defined by `defSetting()`.  `parseSettings()` validates existing
values and adds missing values to `settings`, updating the object in place.

## `parser.settingsUsage()` (server)

`settingsUsage()` returns a usage string that describes the previous calls to
`defSetting()`.
