import { createLdapClient } from 'meteor/nog-ldap';
import { check, Match } from 'meteor/check';

const matchDev = Match.Where((x) => {
  check(x, String);
  return x === 'dev';
});

const matchLdapUrl = Match.Where((x) => {
  check(x, String);
  return x.startsWith('ldap://') || x.startsWith('ldaps://');
});

const matchLdapUrls = Match.Where((x) => {
  check(x, [matchLdapUrl]);
  return x.length > 0;
});

const matchOneLdap = Match.Where((x) => {
  check(x, {
    service: String,
    urls: matchLdapUrls,
    groupDn: String,
    userDn: String,
    autoRegisterGroups: [String],
  });
  return true;
});

const matchLdap = [matchOneLdap];

const matchDevLdap = Match.OneOf(matchDev, matchLdap);

const ldapSetting = {
  key: 'ldap',
  val: [],
  help: `
\`ldap\` controls how user group information is retrieved from LDAP.  It is a
list of configuration objects that corresponding to account services, like
 \`gitzib\`.

    Meteor.settings.ldap: [
      {
        service: 'gitexample',
        urls: [
          'ldap://ldap.example.com',
        ],
        groupDn: 'ou=group,dc=example,dc=com',
        userDn: 'ou=People,dc=example,dc=com',
        autoRegisterGroups: ['users'],
      },
    ]

Multiple \`urls\` may be provided to prepare for redundant LDAP connections in
the future.  But only the first URL is currently used.

Accounts with an LDAP group listed in \`autoRegisterGroups\` will be
automatically assigned role \`users\`.

The special value \`ldap: "dev"\` can be used to enable dev settings.
`,
  match: matchDevLdap,
};

function isEqualLdapgroups(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}

function updateUserFromLdapFunc({
  users, settingsLdap,
}) {
  const settingsByService = new Map(
    settingsLdap.map(s => [s.service, s]),
  );

  function updateUserFromLdap({ user }) {
    const $set = {};
    let autoRegisterUser = false;

    Object.entries(user.services).forEach(([k, srv]) => {
      const cfg = settingsByService.get(k);
      if (!cfg) {
        return;
      }

      const { username } = srv;
      if (!username) {
        return;
      }

      const ldap = createLdapClient({
        url: cfg.urls[0],
        groupDn: cfg.groupDn,
        userDn: cfg.userDn,
      });
      if (!ldap.connected) {
        return;
      }

      try {
        const ldapUsers = ldap.searchUser(username);
        if (ldapUsers.length === 0) {
          console.error(
            `[nog-accounts] Missing user \`${username}\` `
            + `in LDAP \`${ldap.url}\`.`,
          );
          return;
        } if (ldapUsers.length > 1) {
          console.error(
            `[nog-accounts] Ambiguous LDAP lookup user \`${username}\` `
            + `in LDAP \`${ldap.url}\`.`,
          );
          return;
        }
        const [ldapUser] = ldapUsers;

        const grps = ldap.searchGroups(username);
        const primaryGrp = ldap.resolveGid(ldapUser.gidNumber);
        if (!grps.includes(primaryGrp)) {
          grps.push(primaryGrp);
        }
        grps.sort();
        if (isEqualLdapgroups(srv.ldapgroups, grps)) {
          return;
        }
        $set[`services.${k}.ldapgroups`] = grps;

        const { autoRegisterGroups = [] } = cfg;
        for (const autoG of autoRegisterGroups) {
          if (grps.includes(autoG)) {
            autoRegisterUser = true;
            return;
          }
        }
      } catch (err) {
        console.error(
          `[nog-accounts] LDAP lookup user \`${username}\` `
          + `in \`${ldap.url}\` failed.`,
          'err', err,
        );
      } finally {
        ldap.destroy();
      }
    });

    if (Object.keys($set).length === 0) {
      return;
    }

    const mod = { $set };
    if (autoRegisterUser) {
      mod.$addToSet = { roles: 'users' };
    }
    users.update(user._id, mod);
    console.log(
      '[nog-accounts] Updated user from LDAP.',
      'userId', user._id,
      'username', user.username,
      'modifier', JSON.stringify(mod),
    );
  }

  return updateUserFromLdap;
}

export {
  ldapSetting,
  updateUserFromLdapFunc,
};
