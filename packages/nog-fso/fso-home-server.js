import crypto from 'crypto';
import { Meteor } from 'meteor/meteor';
import { Match, check } from 'meteor/check';
import {
  AA_FSO_HOME,
  CollNameHomeLinks,
  KeyPath,
  KeyRoute,
  PubNameHome,
} from './fso-home.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';

const matchPrincipal = Match.Where((x) => {
  check(x, String);
  return !!x.match(/^(username|ldapgroup):[^:]+$/);
});

const matchLink = Match.Where((x) => {
  const availableRoutes = [
    'catalog',
    'help',
    'listing',
    'syssug',
    'untracked',
  ];
  check(x, {
    route: String,
    path: String,
  });
  return availableRoutes.includes(x.route);
});

const matchHomeEntry = Match.Where((x) => {
  check(x, {
    description: String,
    principals: [matchPrincipal],
    links: [matchLink],
  });
  return true;
});

const matchFsoHomes = [matchHomeEntry];

function nameHashId(name) {
  const s = crypto.createHash('sha1').update(name, 'utf8').digest('base64');
  // Shorten and replace confusing characters.
  return s.substr(0, 20).replace(/[=+/]/g, 'x');
}

// Home uses client-only collections, see `fso-home-client.js`.
function createCollections() {
  return {};
}

// `publishHome()` publishes links from the `homes` setting that match the
// current users.
function publishHomeFunc({
  namespace, testAccess, homes,
}) {
  const homeLinksCollName = makeCollName(namespace, CollNameHomeLinks);

  return function publishHome() {
    const user = Meteor.user();
    if (!testAccess(user, AA_FSO_HOME, { path: '/' })) {
      this.ready();
      return null;
    }

    const aliases = [`username:${user.username}`];
    Object.values(user.services).forEach(({ ldapgroups }) => {
      if (ldapgroups) {
        aliases.push(...ldapgroups.map(g => `ldapgroup:${g}`));
      }
    });

    function isActiveHome(h) {
      for (const pr of h.principals) {
        if (aliases.includes(pr)) {
          return true;
        }
      }
      return false;
    }

    homes.forEach((h) => {
      if (!isActiveHome(h)) {
        return;
      }
      h.links.forEach((link) => {
        const { route, path } = link;
        const id = nameHashId(`${route}:${path}`);
        this.added(homeLinksCollName, id, {
          [KeyRoute]: route,
          [KeyPath]: path,
        });
      });
    });

    this.ready();
    return null;
  };
}

function registerPublications({
  publisher, namespace, testAccess, homes,
}) {
  function defPub(name, fn) {
    publisher.publish(makePubName(namespace, name), fn);
  }
  defPub(PubNameHome, publishHomeFunc({
    namespace, testAccess, homes,
  }));
}

function createFsoHomeModuleServer({
  namespace, checkAccess, testAccess, publisher, homes,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(checkAccess, Function);
  check(publisher, Match.ObjectIncluding({ publish: Function }));
  check(homes, matchFsoHomes);

  registerPublications({
    publisher, namespace, testAccess, homes,
  });

  const module = {
    ...createCollections({ namespace }),
  };
  return module;
}

export {
  createFsoHomeModuleServer,
  matchFsoHomes,
};
