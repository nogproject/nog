/* global NogBlob */

import { Meteor } from 'meteor/meteor';
import { createBucketRouterFromSettings } from 'meteor/nog-multi-bucket';


NogBlob.bucketRouter = createBucketRouterFromSettings({
  settings: Meteor.settings.multiBucket,
  namespace: { coll: 'nogcontent' },
});

if (Meteor.isTest) {
  console.log('[nog-blob] Bucket health checks are inactive in test mode.');
} else {
  NogBlob.bucketRouter.startChecks();
  console.log('[nog-blob] Started bucket health checks.');
}
