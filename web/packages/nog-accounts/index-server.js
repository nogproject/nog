import { createUserFuncGitimp } from './imp.js';
import { createUserFuncGitzib } from './zib.js';
import {
  ldapSetting,
  updateUserFromLdapFunc,
} from './ldap.js';
import {
  createGitlabClientIdSetting,
  createGitlabClientSecretSetting,
  oauthSecretKeySetting,
} from './settings.js';
import {
  createWellknownAccountsHandler,
  createWellknownAccountsSetting,
} from './wellknown.js';
import {
  fsoUnixDomainsSetting,
  updateUserFromFsoUnixDomainsFunc,
} from './fso-unix-domain.js';


export {
  createGitlabClientIdSetting,
  createGitlabClientSecretSetting,
  createUserFuncGitimp,
  createUserFuncGitzib,
  createWellknownAccountsHandler,
  createWellknownAccountsSetting,
  fsoUnixDomainsSetting,
  ldapSetting,
  oauthSecretKeySetting,
  updateUserFromFsoUnixDomainsFunc,
  updateUserFromLdapFunc,
};
