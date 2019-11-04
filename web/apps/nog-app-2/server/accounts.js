// Before making changes here, consider changing and testing
// `oidc-testapp/server/main.js` first.

import { Meteor } from 'meteor/meteor';
import { Accounts } from 'meteor/accounts-base';
import { OAuthEncryption } from 'meteor/oauth-encryption';
import { ServiceConfiguration } from 'meteor/service-configuration';
import { Oidc } from 'meteor/oidc';
import { nogthrow } from 'meteor/nog-error-2';
import {
  createGitlabClientIdSetting,
  createGitlabClientSecretSetting,
  createUserFuncGitimp,
  createUserFuncGitzib,
  createWellknownAccountsHandler,
  createWellknownAccountsSetting,
  fsoUnixDomainsSetting,
  ldapSetting,
  oauthSecretKeySetting,
  updateUserFromFsoUnixDomainsFunc,
  updateUserFromLdapFunc,
} from 'meteor/nog-accounts';

const serviceNames = ['gitimp', 'gitzib'];

function defAccountsSettings({ parser }) {
  parser.defSetting(createWellknownAccountsSetting({ serviceNames }));
  parser.defSetting(createGitlabClientIdSetting('GITIMP_CLIENT_ID'));
  parser.defSetting(createGitlabClientSecretSetting('GITIMP_CLIENT_SECRET'));
  parser.defSetting(createGitlabClientIdSetting('GITZIB_CLIENT_ID'));
  parser.defSetting(createGitlabClientSecretSetting('GITZIB_CLIENT_SECRET'));
  parser.defSetting(oauthSecretKeySetting);
  parser.defSetting(ldapSetting);
  parser.defSetting(fsoUnixDomainsSetting);
}

function getSettingsWellknownAccounts() {
  const settings = Meteor.settings.wellknownAccounts;
  if (settings !== 'dev') {
    return settings;
  }

  // Add `gittest` for `handleWellknownAccounts()` in order to alias `bzfproha`
  // to gittest `bob` aka Meteor user `sprohaska`; see `createTestingUser()` in
  // `./minifso.js`.
  serviceNames.push('gittest');

  const devSettings = [
    {
      gittest: { username: 'bob' },
      gitimp: { username: 'bzfproha' },
      gitzib: { username: 'bzfproha' },
    },
    {
      gitimp: { username: 'bzfhombe' },
      gitzib: { username: 'bzfhombe' },
    },
  ];
  console.log(
    '[nog-app-2] Using wellknownAccounts dev settings:', devSettings,
  );
  return devSettings;
}

function getSettingsLdap() {
  const settings = Meteor.settings.ldap;
  if (settings !== 'dev') {
    return settings;
  }

  const devSettings = [
    {
      service: 'gitimp',
      urls: [
        'ldap://localhost:13389',
      ],
      groupDn: 'ou=Groups,ou=mi,dc=imp,dc=fu-berlin,dc=de',
      userDn: 'ou=People,ou=mi,dc=imp,dc=fu-berlin,dc=de',
      autoRegisterGroups: [],
    },
    {
      service: 'gitzib',
      urls: [
        'ldap://localhost:14389',
      ],
      groupDn: 'ou=group,dc=zib,dc=de',
      userDn: 'ou=People,dc=zib,dc=de',
      autoRegisterGroups: [],
    },
  ];
  console.log('[nog-app-2] Using ldap dev settings:', devSettings);
  console.log(
    '[nog-app-2] Use an SSH tunnel for testing gitimp: '
    + 'ssh -L localhost:13389:ldap.imp.fu-berlin.de:389 login.imp',
  );
  console.log(
    '[nog-app-2] Use an SSH tunnel for testing gitzib: '
    + 'ssh -L localhost:14389:tyr1.zib.de:389 login.zib',
  );
  return devSettings;
}

function getSettingsFsoUnixDomains() {
  const settings = Meteor.settings.fsoUnixDomains;
  if (settings !== 'dev') {
    return settings;
  }

  const devSettings = [
    {
      service: 'gittest',
      domain: 'EXDOM',
    },
  ];
  console.log('[nog-app-2] Using FSO Unix domains dev settings:', devSettings);
  return devSettings;
}

const sanitizedErrCreateAccount = {
  errorCode: 'ERR_CREATE_ACCOUNT',
  reason: (
    'Failed to create account due to an internal error.  '
    + 'You should contact an administrator.'
  ),
};

const ERR_CREATE_ACCOUNT_UNKNOWN_SERVICE = {
  errorCode: 'ERR_CREATE_ACCOUNT_UNKNOWN_SERVICE',
  statusCode: 401,
  sanitized: sanitizedErrCreateAccount,
  reason: 'Cannot create account: unknown service details.',
};

function initAccounts({ fsoUnixDomains }) {
  const {
    GITIMP_CLIENT_ID,
    GITIMP_CLIENT_SECRET,
    GITZIB_CLIENT_ID,
    GITZIB_CLIENT_SECRET,
    oauthSecretKey,
  } = Meteor.settings;

  Accounts.config({ oauthSecretKey });

  const ConfigServiceGitlabCommon = {
    authorizationEndpoint: '/oauth/authorize',
    loginStyle: 'popup',
    tokenEndpoint: '/oauth/token',
    userinfoEndpoint: '/oauth/userinfo',
    requestPermissions: ['openid', 'read_user'],
    idTokenWhitelistFields: ['name', 'nickname'],
  };

  const ConfigServiceGitimp = {
    service: 'gitimp',
    clientId: GITIMP_CLIENT_ID,
    secret: OAuthEncryption.seal(GITIMP_CLIENT_SECRET),
    serverUrl: 'https://git.imp.fu-berlin.de',
    ...ConfigServiceGitlabCommon,
  };

  const ConfigServiceGitzib = {
    service: 'gitzib',
    clientId: GITZIB_CLIENT_ID,
    secret: OAuthEncryption.seal(GITZIB_CLIENT_SECRET),
    serverUrl: 'https://git.zib.de',
    ...ConfigServiceGitlabCommon,
  };

  const serviceConfigs = [
    ConfigServiceGitimp,
    ConfigServiceGitzib,
  ];
  serviceConfigs.forEach((cfg) => {
    const { service } = cfg;
    ServiceConfiguration.configurations.upsert({ service }, { $set: cfg });
    Oidc.registerServer(service, {
      usernameFromUserinfo: userinfo => userinfo.nickname,
    });
    Oidc.registerOidcService(service);
  });

  function isKnownUsername(username) {
    return !!Meteor.users.findOne({ username });
  }

  function isKnownEmail(addr) {
    return !!Meteor.users.findOne({ 'emails.address': addr });
  }

  const handleWellknownAccounts = createWellknownAccountsHandler({
    users: Meteor.users,
    serviceNames,
    settings: getSettingsWellknownAccounts(),
  });

  const createUserFuncs = [
    createUserFuncGitimp({
      isKnownUsername, isKnownEmail,
      service: ConfigServiceGitimp.service,
      gitlabUrl: ConfigServiceGitimp.serverUrl,
      domain: 'fu', subUsernames: ['oidc-testapp'],
    }),
    createUserFuncGitzib({
      isKnownUsername, isKnownEmail,
      service: ConfigServiceGitzib.service,
      gitlabUrl: ConfigServiceGitzib.serverUrl,
      domain: 'zib', subUsernames: ['oidc-testapp'],
    }),
  ];

  Accounts.onCreateUser((opts, partialUser) => {
    // XXX Can be useful for debugging:
    //
    // ```
    // console.log('onCreateUser() opts:', opts);
    // console.log('onCreateUser() partialUser:', partialUser);
    // ```

    // `handleWellknownAccounts()` either does nothing, or it adds the
    // `partialUser` to an existing user and throws an exception that tells the
    // client to login again.  The next login will resolve to the existing
    // user.
    handleWellknownAccounts(partialUser);

    let user = null;
    for (const createUser of createUserFuncs) {
      user = createUser(opts, partialUser);
      if (user) {
        break;
      }
    }
    if (!user) {
      nogthrow(ERR_CREATE_ACCOUNT_UNKNOWN_SERVICE);
    }

    // XXX maybe more input validation.
    const profile = {};
    if (opts.profile) {
      const { name, nickname } = opts.profile;
      if (name) {
        profile.name = name;
      }
      if (nickname && nickname !== name) {
        profile.nickname = nickname;
      }
    }
    user.profile = profile;

    return user;
  });

  if (getSettingsLdap().length > 0) {
    const updateUserFromLdap = updateUserFromLdapFunc({
      users: Meteor.users,
      settingsLdap: getSettingsLdap(),
    });

    Accounts.onLogin((opts) => {
      const { user } = opts;
      updateUserFromLdap({ user });
    });
  }

  if (fsoUnixDomains.domainConns.length > 0) {
    const updateUserFromFsoUnixDomains = updateUserFromFsoUnixDomainsFunc({
      users: Meteor.users,
      fsoUnixDomains,
      settingsFsoUnixDomains: getSettingsFsoUnixDomains(),
    });

    Accounts.onLogin((opts) => {
      const { user } = opts;
      updateUserFromFsoUnixDomains({ user });
    });
  }
}

export {
  defAccountsSettings,
  initAccounts,
};
