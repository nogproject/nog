/* global Package */
import { Meteor } from 'meteor/meteor';
import 'meteor/nog-settings';
import { NogContent } from 'meteor/nog-content';
import { createRateLimiter } from 'meteor/nog-search';
import {
  createCatalogServerModule,
  matchCatalogConfig,
} from './nog-catalog-main-server.js';
import { namespace } from './default-namespace.js';


function defaultAccess() {
  const pkg = Package['nog-access'];
  if (pkg) {
    const { NogAccess } = pkg;
    console.log('[nog-catalog] using nog-access default policy.');
    return {
      checkAccess: NogAccess.checkAccess,
      testAccess: NogAccess.testAccess,
    };
  }
  console.log(
    '[nog-catalog] default access control disabled, ' +
    'since nog-access is not available.',
  );
  return {
    checkAccess() {},
    testAccess() {
      return true;
    },
  };
}


function defaultRateLimiters() {
  const readLimiter = createRateLimiter({
    name: 'catalogUpdateReads',
    maxOps: Meteor.settings.catalogUpdateReadRateLimit,
    intervalMs: 1000,
  });
  const writeLimiter = createRateLimiter({
    name: 'catalogUpdateWrites',
    maxOps: Meteor.settings.catalogUpdateWriteRateLimit,
    intervalMs: 1000,
  });
  return { readLimiter, writeLimiter };
}


function createDefaultModule() {
  const { checkAccess, testAccess } = defaultAccess();
  const rateLimiters = defaultRateLimiters();
  return createCatalogServerModule({
    namespace,
    checkAccess, testAccess,
    contentStore: NogContent.store,
    rateLimiters,
  });
}


const NogCatalog = (
  Meteor.settings.public.optCatalog === 'disabled' ?
    null : createDefaultModule()
);
if (!NogCatalog) {
  console.log('[nog-catalog] disabled.');
}


export {
  NogCatalog,
  createCatalogServerModule,
  matchCatalogConfig,
};
