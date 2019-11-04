/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import {
  createGitlabClientIdSetting,
  createGitlabClientSecretSetting,
  createUserFuncGitimp,
  createUserFuncGitzib,
  createWellknownAccountsHandler,
  createWellknownAccountsSetting,
  ldapSetting,
  oauthSecretKeySetting,
  updateUserFromLdapFunc,
} from 'meteor/nog-accounts';

describe('nog-accounts server', function () {
  it('has createGitlabClientIdSetting()', function () {
    expect(createGitlabClientIdSetting).to.be.a('function');
  });

  it('has createGitlabClientSecretSetting()', function () {
    expect(createGitlabClientSecretSetting).to.be.a('function');
  });

  it('has createUserFuncGitimp()', function () {
    expect(createUserFuncGitimp).to.be.a('function');
  });

  it('has createUserFuncGitzib()', function () {
    expect(createUserFuncGitzib).to.be.a('function');
  });

  it('has ldapSetting', function () {
    expect(ldapSetting).to.be.a('object');
  });

  it('has oauthSecretKeySetting', function () {
    expect(oauthSecretKeySetting).to.be.a('object');
  });

  it('has updateUserFromLdapFunc()', function () {
    expect(updateUserFromLdapFunc).to.be.a('function');
  });

  it('has createWellknownAccountsSetting()', function () {
    expect(createWellknownAccountsSetting).to.be.a('function');
  });

  it('has createWellknownAccountsHandler()', function () {
    expect(createWellknownAccountsHandler).to.be.a('function');
  });
});
