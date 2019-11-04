import { Meteor } from 'meteor/meteor';
import { createSettingsParser } from 'meteor/nog-settings-2';
import {
  createAccessModuleServer,
  checkUserPluginScopesV2,
  principalPluginUsername,
  principalPluginLdapgroups,
} from 'meteor/nog-access-2';
import {
  clusterOptSingleInstanceModeSetting,
  createClusterModuleServer,
} from 'meteor/nog-cluster-2';

import {
  NsAccess,
  NsFsoMiniRegistry,
  NsCluster,
  NsReadyJwts,
} from '../imports/namespace.js';
import {
  createTestingUsers,
  publicTestsPasswordsUserSetting,
} from './testing-users.js';
import { minifsoSetting } from './minifso-settings.js';
import { initMinifso } from './minifso.js';

const parser = createSettingsParser();
parser.defSetting(publicTestsPasswordsUserSetting);
parser.defSetting(clusterOptSingleInstanceModeSetting);
parser.defSetting(minifsoSetting);
console.log('[fso-testapp] Meteor.settings:\n');
console.log(parser.settingsUsage());
parser.parseSettings(Meteor.settings);

const NogAccess = createAccessModuleServer({
  namespace: NsAccess,
  users: Meteor.users,
  checkUserPlugins: [
    checkUserPluginScopesV2,
  ],
  principalPlugins: [
    principalPluginUsername,
    principalPluginLdapgroups,
  ],
});

const NogCluster = createClusterModuleServer({
  namespace: NsCluster,
  optSingleInstanceMode: Meteor.settings.cluster.optSingleInstanceMode,
  optGlobalReadOnly: Meteor.settings.optGlobalReadOnly,
});

const testingUsers = createTestingUsers();

const {
  NogFsoMiniRegistry, // eslint-disable-line no-unused-vars
  NogReadyJwts, // eslint-disable-line no-unused-vars
} = initMinifso({
  NsFsoMiniRegistry, NsReadyJwts,
  testingUsers, NogAccess, NogCluster,
});

console.log('[fso-testapp] Started.');
