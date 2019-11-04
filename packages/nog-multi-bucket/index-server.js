// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.

import './package-peer-versions.js';

import { createBucketRouterFromSettings } from './bucket-router.js';
import { matchMultiBucketSettings } from './multi-bucket-settings.js';

export {
  createBucketRouterFromSettings,
  matchMultiBucketSettings,
};
