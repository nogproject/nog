# Package `nog-ops`

The package `nog-ops` contains tooling for operators.

To run all database consistency checks, login with an admin account and execute
the following command from the Browser console:

```
NogOps.call.dbck(function (err, res) { console.log(err, res); });
```

You can select a set of checks as follows:

```
checks = ['blobstatus', 'sha', 'connectivity', 'sharing', 'catalog'];
NogOps.call.dbck({ checks }, function (err, res) { console.log(err, res); });
```

Progress is reported to the server logs.

Be careful when executing the checks in a production environment.  The checks
create high database and CPU load.  A better option might be to run the checks
in a separate admin deployment, maybe with read-only access to the database.
