import ldap from 'ldapjs';
import future from 'fibers/future';


function ldapsearch(client, dn, opts) {
  // The ldapjs source as of 2017-10-11 contains a comment in `client.js` that
  // 'the timed-out request should be abandoned'.  It could mean that ldapjs
  // may emit spurious events after a `timeout`.  We ensure that `fut` is used
  // only once to either `throw()` or `return()` by setting it to `null`
  // afterwards.
  let fut = new future();

  client.search(dn, opts, (err, res) => {
    if (err != null) {
      fut.throw(err);
      fut = null;
      return;
    }

    const items = [];
    res.on('searchEntry', (entry) => {
      items.push(entry.object);
    });
    res.on('searchReference', (referral) => {
      const hosts = referral.uris.join(', ');
      // A LDAP SearchResultReference can occur in large AND distributed
      // directories. It seems not to be the case at ZIB and IMP.
      // See: http://ldapwiki.com/wiki/SearchResultReference
      // However, it is recommended to handle it even with an empty function.
      // See: https://github.com/mcavage/node-ldapjs/issues/263
      console.log(`[nog-ldap] Warning: ignored search reference ${hosts}.`);
    });
    res.on('error', (err2) => {
      if (!fut) {
        return;
      }
      fut.throw(err2);
      fut = null;
    });
    res.on('timeout', () => {
      if (!fut) {
        return;
      }
      fut.throw(new Error('Request timeout'));
      fut = null;
    });
    res.on('end', (result) => {
      if (!fut) {
        return;
      }
      if (result.status !== 0) {
        const err2 = new Error(`ldap status ${result.status}.`);
        fut.throw(err2);
        fut = null;
        return;
      }
      fut.return(items);
      fut = null;
    });
  });

  return fut.wait();
}

function resultCns(results) {
  return results.map(e => e.cn);
}

function createLdapClient(options) {
  const newclient = {
    url: options.url,
    groupFilter: '(objectClass=posixGroup)',
    userFilter: '(objectClass=posixAccount)',
    groupAttr: 'gidNumber',
    userAttr: 'uid',
    groupSearchAttr: 'memberUid',
    groupDn: options.groupDn,
    userDn: options.userDn,
    client: null,

    resolveGid(gid) {
      const opts = {
        filter: `(&${this.groupFilter}(${this.groupAttr}=${gid}))`,
        scope: 'sub',
        attributes: ['cn'],
      };
      return resultCns(ldapsearch(this.client, this.groupDn, opts))[0];
    },

    searchGroups(username) {
      const opts = {
        filter: `(&${this.groupFilter}(${this.groupSearchAttr}=${username}))`,
        scope: 'sub',
        attributes: ['cn'],
      };
      return resultCns(ldapsearch(this.client, this.groupDn, opts));
    },

    searchUser(username) {
      const opts = {
        filter: `(&${this.userFilter}(${this.userAttr}=${username}))`,
        scope: 'sub',
      };
      return ldapsearch(newclient.client, newclient.userDn, opts);
    },

    destroy() {
      if (this.client) {
        this.client.destroy();
      }
      this.client = null;
    },
  };

  // `createClient` tries to connect the server in a new thread. So we
  // wait for the connection to be established or fail.
  const fut = new future();
  newclient.client = ldap.createClient({
    url: options.url,
    connectTimeout: 3000,
    timeout: 3000,
  });
  newclient.client.once('error', (err) => {
    console.error(`[nog-ldap] Failed to connect: ${err}`);
    fut.return(false);
  });
  newclient.client.once('connectTimeout', (err) => {
    console.error(`[nog-ldap] Failed to connect, timeout: ${err}`);
    fut.return(false);
  });
  newclient.client.once('connect', () => {
    fut.return(true);
  });
  newclient.connected = fut.wait();

  return newclient;
}

export {
  createLdapClient,
};
