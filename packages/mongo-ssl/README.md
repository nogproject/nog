# Meteor package `mongo-ssl`

The Meteor package `mongo-ssl` configures SSL for the default MongoDB
connection from environment variables.

The package must be listed in `.meteor/packages` before the first package that
creates a `Mongo.Collection`.

Environment variables and corresponding MongoDB Node.js driver options
<http://mongodb.github.io/node-mongodb-native/2.1/reference/connecting/connection-settings/>:

* `MONGO_SSL_CA`: Concatenated certs in PEM format or a path from which to load
  them.  Configures driver option `sslCA` and `ssl=true`.
* `MONGO_SSL_CERT`: Combined cert and private key, that is `cat cert.pem
  key.pem >combined.pem`, in PEM format or a path from which to load them.
  Configures driver options `sslCert`, `sslKey`, and `ssl=true`.
* `MONGO_SSL_INSECURE`: If set to any nonempty string, SSL validation will be
  skipped.  Configures driver option `sslValidate=false`.
