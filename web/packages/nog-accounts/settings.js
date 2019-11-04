import { check, Match } from 'meteor/check';

const matchBytes16Base64 = Match.Where((x) => {
  check(x, String);
  return x.length === 24;
});

const matchHex = Match.Where((x) => {
  check(x, String);
  return x.match(/^[0-9a-f]+$/) && (x.length % 2 === 0);
});

function createGitlabClientIdSetting(key) {
  return {
    key,
    val: null,
    help: `
\`${key}\` is a GitLab Application ID.
`,
    match: matchHex,
  };
}

function createGitlabClientSecretSetting(key) {
  return {
    key,
    val: null,
    help: `
\`${key}\` is a GitLab Application Secret.
`,
    match: matchHex,
  };
}

const oauthSecretKeySetting = {
  key: 'oauthSecretKey',
  val: null,
  help: `
\`oauthSecretKey\` is the secret for encrypting OAuth access tokens before
storing them to MongoDB.  Valid keys can be created with:

    node -p 'require("crypto").randomBytes(16).toString("base64")'

`,
  match: matchBytes16Base64,
};

export {
  createGitlabClientIdSetting,
  createGitlabClientSecretSetting,
  oauthSecretKeySetting,
};
