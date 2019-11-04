import { defSetting, matchNonNegativeNumber } from 'meteor/nog-settings';


defSetting({
  key: 'cache.resolveImgSrc.expireAfterSeconds',
  val: 10 * 24 * 60 * 60,
  help: `
\`resolveImgSrc.expireAfterSeconds\` is the TTL for the ref path resolve cache
in \`resolveImgSrc\`.
`,
  match: matchNonNegativeNumber,
});
