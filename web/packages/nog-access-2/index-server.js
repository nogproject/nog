// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.
import './package-peer-versions.js';

import { check, Match } from 'meteor/check';
import { defMethodCalls } from './methods.js';
import { StatementsIsRoleX } from './statements.js';
import {
  checkUserPluginScopesV2,
  checkUserPluginsDefault,
  principalPluginRoles,
  principalPluginUsername,
  principalPluginUserid,
  principalPluginLdapgroups,
  principalPluginsDefault,
  createAuthorizer,
} from './authz.js';
import { testingDescribeStatements } from './testing.js';

function createAccessModuleServer({
  namespace, users, checkUserPlugins, principalPlugins,
}) {
  check(namespace, { meth: String });
  check(users, Match.Any); // `Mongo.Collection`.
  check(checkUserPlugins, Match.Optional([Function]));
  check(principalPlugins, Match.Optional([Function]));

  const authz = createAuthorizer({
    users,
    checkUserPlugins: checkUserPlugins || checkUserPluginsDefault,
    principalPlugins: principalPlugins || principalPluginsDefault,
  });

  const module = {
    addStatement: authz.addStatement.bind(authz),
    addStatements: authz.addStatements.bind(authz),
    checkAccess: authz.checkAccess.bind(authz),
    testAccess: authz.testAccess.bind(authz),
  };
  // Register Meteor methods without assigning `callX()` functions to `module`.
  // Server code should call the real functions, not via a Meteor method.
  defMethodCalls(module, { namespace });
  return module;
}

export {
  createAccessModuleServer,
  checkUserPluginScopesV2,
  principalPluginRoles,
  principalPluginUsername,
  principalPluginUserid,
  principalPluginLdapgroups,
  StatementsIsRoleX,
  testingDescribeStatements,
};
