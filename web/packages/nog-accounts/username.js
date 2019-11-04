import { Random } from 'meteor/random';

function isBlacklisted(name) {
  const blacklist = [
    /^about.*/,
    /^admin.*/,
    /^api.*/,
    /^blog.*/,
    /^contact.*/,
    /^help.*/,
    /^impressum.*/,
    /^nog.*bot.*/,
    /^root.*/,
    /^search.*/,
    /^security.*/,
    /^settings.*/,
    /^site.*/,
    /^tools.*/,
    /^user$/,
    /^zib.*/,
  ];
  for (const b of blacklist) {
    if (name.match(b)) {
      return true;
    }
  }
  return false;
}

function simpleUsername(name) {
  return name.toLowerCase().replace(/[^a-z0-9]/g, '');
}

// `uniqueUsername()` create a unique username that is safe, simple if
// possible, and unused according to `isKnownUsername(username)`.  It uses the
// following heuristics:
//
//  - Keeps only whitelisted characters from the `domainUsername`.
//  - Prefix short or blacklisted usernames with `user`.
//  - Add a domain suffix.
//  - Add a random hex.
//
function uniqueUsername({
  isKnownUsername,
  domainUsername,
  domain,
}) {
  let name = simpleUsername(domainUsername);
  if (name === '') {
    name = `user0${Random.hexString(2)}`;
  } else if (name.length < 3 || isBlacklisted(name)) {
    name = `user${name}`;
  }

  let username = name;
  if (!isKnownUsername(username)) {
    return username;
  }

  username = `${name}_${domain}`;
  let n = 2;
  while (isKnownUsername(username)) {
    username = `${name}1${Random.hexString(n)}_${domain}`;
    n += 1;
  }
  return username;
}

export {
  uniqueUsername,
};
