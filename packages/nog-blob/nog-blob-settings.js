import {
  defSetting,
  matchNonNegativeNumber,
  matchPositiveNumber,
} from 'meteor/nog-settings';
import { matchMultiBucketSettings } from 'meteor/nog-multi-bucket';


defSetting({
  key: 'public.upload.concurrentUploads',
  val: 3,
  help: `
\`concurrentUploads\` limits the number of concurrent file uploads from a
browser.
`,
  match: matchPositiveNumber,
});

defSetting({
  key: 'public.upload.concurrentPuts',
  val: 10,
  help: `
\`concurrentPuts\` limits the number of concurrent HTTP PUT requests when
uploading to S3.
`,
  match: matchPositiveNumber,
});

defSetting({
  key: 'public.upload.concurrentPutsSafari',
  val: 4,
  help: `
\`concurrentPutsSafari\` limits the number of concurrent HTTP PUT requests when
uploading to S3 from Safari.  A number < 5 is recommended to avoid spurious
ETag errors.
`,
  match: matchPositiveNumber,
});

defSetting({
  key: 'public.upload.uploadRetries',
  val: 9,
  help: `
\`uploadRetries\` configures how often upload requests to S3 are retried.
`,
  match: matchNonNegativeNumber,
});

defSetting({
  key: 'multiBucket',
  val: {
    readPrefs: ['noglocal'],
    writePrefs: ['noglocal'],
    fallback: 'noglocal',
    buckets: [
      {
        name: 'noglocal',
        endpoint: 'http://localhost:10080',
        accessKeyId: 'Cdemo',
        secretAccessKey: 'Cdemosecret',
      },
    ],
  },
  help: `
\`multiBucket\` configures uploads to S3.  See package \`nog-multi-bucket\` for
details.
`,
  match: matchMultiBucketSettings,
});
