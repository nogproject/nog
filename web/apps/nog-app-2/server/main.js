import { Meteor } from 'meteor/meteor';
import { createSettingsParser } from 'meteor/nog-settings-2';
import {
  createAccessModuleServer,
  checkUserPluginScopesV2,
  principalPluginUserid,
  principalPluginUsername,
  principalPluginLdapgroups,
} from 'meteor/nog-access-2';
import {
  clusterOptSingleInstanceModeSetting,
  createClusterModuleServer,
} from 'meteor/nog-cluster-2';

import {
  NsAccess,
  NsCluster,
  NsFsoMiniRegistry,
  NsReadyJwts,
} from '../imports/namespaces.js';
import {
  defAccountsSettings,
  initAccounts,
} from './accounts.js';
import {
  minifsoSetting,
  initMinifso,
} from './minifso.js';
import {
  initUnixSocketMode,
} from './listen.js';

const fmtUsage = ({ settings }) => `\

========================================

Available settings, from \`defSetting()\`:

${settings}

`;

const parser = createSettingsParser();
defAccountsSettings({ parser });
parser.defSetting(clusterOptSingleInstanceModeSetting);
parser.defSetting(minifsoSetting);

parser.defSetting({
  key: 'optGlobalReadOnly',
  val: false,
  help: `
\`optGlobalReadOnly=true\` disables certain code paths, so that the app works
to some extend with read-only access to MongoDB.
`,
  match: Boolean,
});

if (process.env.NOG_SETTINGS_HELP) {
  console.log(fmtUsage({ settings: parser.settingsUsage() }));
  process.exit(0);
}
parser.parseSettings(Meteor.settings);

const NogAccess = createAccessModuleServer({
  namespace: NsAccess,
  users: Meteor.users,
  checkUserPlugins: [
    checkUserPluginScopesV2,
  ],
  principalPlugins: [
    principalPluginUserid,
    principalPluginUsername,
    principalPluginLdapgroups,
  ],
});

const NogCluster = createClusterModuleServer({
  namespace: NsCluster,
  optSingleInstanceMode: Meteor.settings.cluster.optSingleInstanceMode,
  optGlobalReadOnly: Meteor.settings.optGlobalReadOnly,
});

const { fsoUnixDomains } = initMinifso({
  NsFsoMiniRegistry, NsReadyJwts,
  NogAccess, NogCluster,
});

initAccounts({
  fsoUnixDomains,
});

initUnixSocketMode();

console.log('[nog-app-2] Started.');
