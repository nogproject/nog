# Package `nog-ldap`

This package enables the communication with LDAP servers.
It supports the search for a user and for groups containing a given user.


## Usage

`NogLdap.newClient(options)` creates a new connection to an LDAP server. The
`options` object should at least contain the property `url` with the LDAP url
as value, and the properties `groupDn` and `userDn`, which specify the base DN
for group and user search, respectively. Other options and their defaults are:

```
groupFilter: '(objectClass=posixGroup)'
userFilter: '(objectClass=posixAccount)'
groupAttr: 'gidNumber'
userAttr: 'uid'
groupSearchAttr: 'memberUid'
```

The `client` object returned by `NogLdap.newClient(options)` has the following
functions:

 - `client.connected`: Returns whether or not the connection is established.
    Should be checked before performing searches.
 - `client.searchUser(user)`: Searches for entries in `userDn`, that fulfill
    the `userFilter` and `userAttr=user`, returns an array of entry objects.
 - `client.searchGroups(user)`: Searches for entries in `groupDn`, that fulfill
    the `groupFilter` and `groupSearchAttr=user`, returns an array of strings
    with group names.
 - `client.resolveGid(gid)`: Searches for entries in `groupDn`, that fulfill
    the `groupFilter` and `groupAttr=gid`, returns a string with the groups name.
    This is useful in cases, where users are not listed in their main group.
