import { check, Match } from 'meteor/check';

const matchDev = Match.Where((x) => {
  check(x, String);
  return x === 'dev';
});

const matchOneFsoUnixDomain = Match.Where((x) => {
  check(x, {
    service: String,
    domain: String
  });
  return true;
});

const matchFsoUnixDomains = [matchOneFsoUnixDomain];

const matchFsoUnixDomainsDev = Match.OneOf(matchDev, matchFsoUnixDomains);

const fsoUnixDomainsSetting = {
  key: 'fsoUnixDomains',
  val: [],
  help: `
\`fsoUnixDomains\` controls how user group information is retrieved from FSO
Unix domains.  It is a list of configuration objects that corresponding to
account services, like \`gitzib\`:

    Meteor.settings.fsoUnixDomains: [
      {
        service: 'gitexample',
        domain: 'EXDOM',
      },
    ]

The domains must also be configured in setting \`minifso\`.

The special value \`fsoUnixDomains: "dev"\` can be used to enable dev settings.
`,
  match: matchFsoUnixDomainsDev,
};

function isEqualLdapgroups(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}

function updateUserFromFsoUnixDomainsFunc({
  users, fsoUnixDomains, settingsFsoUnixDomains,
}) {
  const { sysCallCreds } = fsoUnixDomains;

  const domainConns = new Map(
    fsoUnixDomains.domainConns.map(c => [c.domain, c.conn]),
  )

  const settingsByService = new Map(
    settingsFsoUnixDomains.map(s => {
      return [s.service, {
        ...s, conn: domainConns.get(s.domain),
      }];
    }),
  );

  function updateUserFromFsoUnixDomains({ user }) {
    const $set = {};

    Object.entries(user.services).forEach(([k, srv]) => {
      const cfg = settingsByService.get(k);
      if (!cfg) {
        return;
      }

      const { username } = srv;
      if (!username) {
        return;
      }

      const c = cfg.conn.unixDomainsClient(sysCallCreds);
      try {
        const user = c.getUnixUserSync({
          domainName: cfg.domain,
          user: username,
        })
        const { groups } = user;
        groups.sort();
        if (isEqualLdapgroups(srv.ldapgroups, groups)) {
          return;
        }
        $set[`services.${k}.ldapgroups`] = groups;
      } catch (err) {
        console.error(
          `[nog-accounts] FSO Unix domain lookup user \`${username}\` `
          + `in domain \`${cfg.domain}\` failed.`,
          'err', err,
        );
      }
    });

    if (Object.keys($set).length === 0) {
      return;
    }

    const mod = { $set };
    users.update(user._id, mod);
    console.log(
      '[nog-accounts] Updated user from FSO Unix domains.',
      'userId', user._id,
      'username', user.username,
      'modifier', JSON.stringify(mod),
    );
  }

  return updateUserFromFsoUnixDomains;
}

export {
  fsoUnixDomainsSetting,
  updateUserFromFsoUnixDomainsFunc,
};
