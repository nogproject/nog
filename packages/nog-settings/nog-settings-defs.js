// `defSetting()` is usually placed in a common dependency ancestor package, so
// that it is initialized first.  All other packages that use the setting
// depend on the ancestor directly or indirectly.  If there is no such common
// dependency ancestor package, the `defSetting()` can be placed here.  All
// packages that rely on the setting should depend on `nog-settings` even if
// they do not directly import symbols from it.

import { defSetting } from './nog-settings.js';
import { matchNonNegativeNumber } from './nog-settings-match.js';


// `nog-access` and `nog-files` use `uploadSizeLimit`.  But `nog-access` is
// optional, so there is no common dependency ancestor package.

defSetting({
  key: 'public.upload.uploadSizeLimit',
  val: 20 * 1024 * 1024 * 1024,
  help: `
\`uploadSizeLimit\` limits the blob size.  Use 0 to disable the limit.
`,
  match: matchNonNegativeNumber,
});


defSetting({
  key: 'optGlobalReadOnly',
  val: false,
  help: `
\`optGlobalReadOnly=true\` disables certain code paths, so that Nog works to
some extend with read-only access to MongoDB.
`,
  match: Boolean,
});


// `optCatalog` is in `nog-settings`, because it may be needed at various
// places to control the UI.

defSetting({
  key: 'public.optCatalog',
  val: 'disabled',
  help: `
\`optCatalog\` controls how the package \`nog-catalog\` is exposed:

 - "disabled": The catalog code is inactive.
 - "hidden": Catalogs are available at a hidden URL.
 - "enabled": Catalogs are visible to all users.

`,
  match: String,
});
