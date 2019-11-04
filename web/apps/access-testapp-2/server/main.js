import { Meteor } from 'meteor/meteor';
import {
  createAccessModuleServer,
  checkUserPluginScopesV2,
  principalPluginRoles,
  principalPluginUsername,
  principalPluginUserid,
  principalPluginLdapgroups,
  StatementsIsRoleX,
} from 'meteor/nog-access-2';
import { createSettingsParser } from 'meteor/nog-settings-2';

import { NsAccess } from '../imports/namespace.js';
import { createUsers, createUsersSetting } from './users.js';

const parser = createSettingsParser();
parser.defSetting(createUsersSetting);
console.log('Meteor.settings:\n');
console.log(parser.settingsUsage());
parser.parseSettings(Meteor.settings);

const NogAccess = createAccessModuleServer({
  namespace: NsAccess,
  users: Meteor.users,
  checkUserPlugins: [
    checkUserPluginScopesV2,
  ],
  principalPlugins: [
    principalPluginRoles,
    principalPluginUsername,
    principalPluginUserid,
    principalPluginLdapgroups,
  ],
});
NogAccess.addStatements(StatementsIsRoleX);

Meteor.startup(createUsers);

console.log('[access-testapp-2] Started.');
