// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.
import './package-peer-versions.js';

import { createAuthApiActions } from './auth-api.js';
import { createBearerJwtAuthn } from './bearer-jwt-auth.js';
import { createFsoTokenProvider } from './token-provider.js';
import {
  matchExpiresIn,
  matchScope,
  matchSubuserName,
} from './match.js';

export {
  createAuthApiActions,
  createBearerJwtAuthn,
  createFsoTokenProvider,
  matchExpiresIn,
  matchScope,
  matchSubuserName,
};
