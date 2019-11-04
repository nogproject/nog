/* eslint-env mocha */
/* eslint-disable func-names */
import { Meteor } from 'meteor/meteor';

import {
  describeNogFsoGrpcTests,
} from './nog-fso-grpc-tests.js';
import {
  describeNogFsoMiniRegistryTests,
} from './nog-fso-mini-registry-tests.js';

describe('fso-testapp server', function () {
  if (Meteor.isTest) {
    describeNogFsoGrpcTests();
    describeNogFsoMiniRegistryTests();
  }
});
