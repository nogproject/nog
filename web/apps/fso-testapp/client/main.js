/* global window */

import { Meteor } from 'meteor/meteor';
import { createReadyJwtsModuleClient } from 'meteor/nog-ready-jwts';

import {
  NsReadyJwts,
} from '../imports/namespace.js';

// The `NogReadyJwts` client is only instantiated for completeness.  It cannot
// be tested, because users cannot sign in to `fso-testapp`.
const NogReadyJwts = createReadyJwtsModuleClient({
  namespace: NsReadyJwts,
  subscriber: Meteor,
});
window.NogReadyJwts = NogReadyJwts;
