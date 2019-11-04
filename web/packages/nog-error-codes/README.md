# Package `nog-error-codes`

Package `nog-error-codes` contains errors that are used by multiple packages.

We usually add new errors where they are used for the first time.  If a second
package needs the same errors, we usually duplicate them.  If three or more
packages use the same errors, we consider moving them to a common package, like
`nog-error-codes`.

To locate common errors:

```sh
git grep 'ERR_.*=' | cut -d : -f 2 | sort | uniq -c | sort -n
```
