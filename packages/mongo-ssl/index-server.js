// `mongo-ssl` must be close to the top of `.meteor/packages`, so that the
// additional options are configured before the first `Collection` is created.
// See Meteor forum entry 'Meteor with the new MongoDB 3.2.1 Compose.io'
// <https://goo.gl/8DKb2q> for the general idea.
//
// See MongoDB Node.js Driver 'URI Connection Settings' <https://goo.gl/V5LNF2>
// for Mongo options.

import { Mongo } from 'meteor/mongo';
import fs from 'fs';

// Use lookahead `(?=`, so that each split result contains the BEGIN line.
const beginStr = '-----BEGIN ';
const beginRgx = /(?=-----BEGIN )/;

const opts = {};

// If env looks like PEM, use as is.  Otherwise load from file.
let ca = process.env.MONGO_SSL_CA;
if (ca) {
  if (!ca.startsWith(beginStr)) {
    ca = fs.readFileSync(ca).toString();
  }
  opts.sslCA = ca.split(beginRgx);
  opts.ssl = true;
}

// Cert and private key, that is `cat cert.pem key.pem >combined.pem`.
let cert = process.env.MONGO_SSL_CERT;
if (cert) {
  if (!cert.startsWith(beginStr)) {
    cert = fs.readFileSync(cert).toString();
  }
  [opts.sslCert, opts.sslKey] = cert.split(beginRgx);
  opts.ssl = true;
}

if (opts.ssl) {
  opts.sslValidate = true;
  if (process.env.MONGO_SSL_INSECURE) {
    opts.sslValidate = false;
  }
}

Mongo.setConnectionOptions(opts);
