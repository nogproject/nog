# Transition from API v0 to v1

We implemented API v1 with commit idversion 1 and object idversion 1 to
support:

 - Timezones in commit dates.
 - Fulltext in `object.text`.

All clients must use API v1.  For clients that use nogpy, it should be
sufficient to update nogpy to version 0.0.11 or later and re-publish the
package.  Code that accesses the REST API directly must be slightly modified to
work with the new API (see [apidoc](apidoc.md)).

The transition schedule was:

 - End of Nov 2015: The new API v1 is available at `/api/v1`.  The old API v0
   is available at `/api/v0`, and also at `/api`.  The server by default
   creates new commits and objects still with the old format (idversion 0).

 - Early Dec 2015: All clients should be ported to use API v1.

 - Mid Dec 2015: We switch `/api` to `/api/v1`.  Old clients can still use
   `/api/v0`.  The server continues to use the old format by default.

 - Jan 2016: We drop `/api/v0` and modify the server to use the new format by
   default.  Old clients cannot be used anymore.

We have no plans to migrate existing objects from the old v0 to the new v1
format.  All clients that use API v1 should be prepared to handled the old
format or at least report a format mismatch error.
