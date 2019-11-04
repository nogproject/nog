import forge from 'node-forge';
import fs from 'fs';
import jwt from 'jsonwebtoken';
import { check, Match } from 'meteor/check';
import {
  ERR_AUTH_UNKNOWN_JWT,
  ERR_AUTH_UNKNOWN_USER,
  ERR_INVALID_JWT,
  ERR_MALFORMED_AUTH_HEADER,
  nogthrow,
} from './errors.js';
import { JwtAAToAction } from './jwt-aa-to-action.js'

const AA_API = 'api';

function parseX5c(x5c) {
  try {
    const der = forge.util.decode64(x5c);
    const asn = forge.asn1.fromDer(der);
    const cert = forge.pki.certificateFromAsn1(asn);
    return cert;
  } catch (err) {
    nogthrow(ERR_INVALID_JWT, { reason: `Invalid x5c: ${err.toString()}` });
  }
  return null;
}

function jwtVerifyHeader(token, { ca, ou }) {
  const decoded = jwt.decode(token, { complete: true });
  if (!decoded || !decoded.header || !decoded.header.x5c) {
    nogthrow(ERR_INVALID_JWT, { reason: 'Invalid JWT header.' });
  }

  const cert = parseX5c(decoded.header.x5c);
  try {
    forge.pki.verifyCertificateChain(ca, [cert]);
  } catch (err) {
    nogthrow(ERR_INVALID_JWT, {
      reason: `Invalid x5c cert: ${err.message}`,
    });
  }

  // See <https://tools.ietf.org/html/rfc5280#section-4.2.1.3>
  if (!cert.getExtension('keyUsage').digitalSignature) {
    nogthrow(ERR_INVALID_JWT, {
      reason: 'x5c key usage does not include signing.',
    });
  }

  const certOu = cert.subject.getField('OU');
  if (!certOu || certOu.value !== ou) {
    nogthrow(ERR_INVALID_JWT, { reason: 'Invalid x5c OU.' });
  }

  return forge.pki.certificateToPem(cert);
}

function jwtVerify(token, secret, opts) {
  try {
    return jwt.verify(token, secret, opts);
  } catch (err) {
    nogthrow(ERR_INVALID_JWT, { reason: err.toString() });
  }
  return null;
}

const matchAA = Match.Where((x) => {
  check(x, String);
  return !!JwtAAToAction[x];
});

const matchScEntry = {
  aa: [matchAA],
  p: Match.Optional([String]),
  n: Match.Optional([String]),
};

const matchSc = [matchScEntry];

function decodeScEntry(sc) {
  return {
    actions: sc.aa.map(a => JwtAAToAction[a]),
    paths: sc.p,
    names: sc.n,
  };
}

function decodeSc(sc) {
  if (!Match.test(sc, matchSc)) {
    nogthrow(ERR_INVALID_JWT, { reason: 'Malformed `sc` claim' });
  }
  return sc.map(decodeScEntry);
}

// Wildcards are ignored.  A JWT explicitly must have `AA_API` to be accepted
// in an authorization header.
function apiInScopes(scopes) {
  for (const sc of scopes) {
    if (sc.actions.includes(AA_API)) {
      return true;
    }
  }
  return false;
}

// Simple usernames must have at least 3 characters from a limited set.
const rgxSimpleUsername = /^[a-z0-9_-]{3,}$/;

// Sys subject `sys:<username>+<subuser>(+<details>)`.
const rgxSysSub = /^sys:[a-z0-9_-]+(\+[a-z0-9_-]+){1,2}$/;

const matchSub = Match.Where((x) => {
  check(x, String);
  return rgxSimpleUsername.test(x) || rgxSysSub.test(x);
});

function decodeSub(sub) {
  if (!Match.test(sub, matchSub)) {
    nogthrow(ERR_INVALID_JWT, { reason: 'Malformed `sub`' });
  }
  if (sub.startsWith('sys:')) {
    return {
      username: sub.split(':')[1].split('+')[0],
      sysSubject: sub,
    };
  }
  return {
    username: sub,
  };
}

const beginRgx = /(?=-----BEGIN )/;

function readCa(path) {
  const pem = fs.readFileSync(path).toString();
  const parts = pem.split(beginRgx);
  return forge.pki.createCaStore(parts);
}

function createBearerJwtAuthn(opts) {
  check(opts, {
    users: Match.Any,
    issuer: String,
    audience: String,
    ou: String,
    ca: String,
    testingJtis: [String],
  });
  const {
    users, issuer, audience, ou,
    testingJtis,
  } = opts;
  const ca = readCa(opts.ca);

  function isKnownJwt(user, jti) {
    // Allow dev JWTs that are not stored in `users`.
    if (testingJtis.includes(jti)) {
      console.log(`[jwt] Allow testing JWT \`${jti}\`.`);
      return true;
    }

    if (!user.services || !user.services.nogfsoiam) {
      return false;
    }
    const { jwts } = user.services.nogfsoiam;
    if (!jwts) {
      return false;
    }
    for (const inf of jwts) {
      if (inf.jti === jti) {
        return true;
      }
    }
    return false;
  }

  return {
    authenticateRequest(req) {
      check(req.headers, Match.ObjectIncluding({
        authorization: String,
      }));
      const { authorization } = req.headers;

      if (!authorization.startsWith('Bearer ')) {
        nogthrow(ERR_MALFORMED_AUTH_HEADER);
      }
      const token = authorization.split(' ', 2)[1];
      const keyPem = jwtVerifyHeader(token, { ca, ou });
      const claims = jwtVerify(token, keyPem, {
        algorithms: ['RS256'],
        issuer,
        audience,
      });

      const { jti, sc, sub } = claims;
      if (!sc) {
        nogthrow(ERR_INVALID_JWT, { reason: 'Missing `sc` claim.' });
      }
      const scopesV2 = decodeSc(sc);

      if (!apiInScopes(scopesV2)) {
        nogthrow(ERR_INVALID_JWT, { reason: 'Missing API scope.' });
      }

      const { username, sysSubject } = decodeSub(sub);
      const user = users.findOne({ username });
      if (!user) {
        nogthrow(ERR_AUTH_UNKNOWN_USER);
      }

      if (!isKnownJwt(user, jti)) {
        nogthrow(ERR_AUTH_UNKNOWN_JWT);
      }

      // Use `scopesV2` to distinguish from Nog access key `scopes`.
      user.scopesV2 = scopesV2;
      if (sysSubject) {
        user.sysSubject = sysSubject;
      }
      req.auth = { user };

      return true;
    },

    // `authenticateFromHeaderFunc()` returns a bound function that can be used
    // for `NogRest.authenticateFromHeader`.
    authenticateFromHeaderFunc() {
      return this.authenticateRequest.bind(this);
    },
  };
}

export {
  createBearerJwtAuthn,
};
