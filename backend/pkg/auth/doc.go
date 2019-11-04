/*

Package `auth` contains basic types for authentication and authorization.

Rules for scope-based path authorization:

 - Normalized resource paths have no trailing slash, e.g. a root is named
   `/foo` not `/foo/`; like a directory realpath or an AWS S3 bucket; but
   unlike Vault's listing policy.
 - The only normalized path that ends with slash is the root path `/`.
 - Paths can be normalized in Go by `path.Clean()` aka `slashpath.Clean()`, not
   `filepath.Clean()`.
 - Callers should call authorize with normalized resource paths.
 - Authorizers should normalize paths before checking access.
 - A token that wants to grant access to a path `/foo` and all paths below must
   include two path patterns: `/foo` and `/foo/*`; i.e. `/foo/*` does not match
   `/foo`.

*/
package auth
