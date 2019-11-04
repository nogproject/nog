/* global window */

import { Meteor } from 'meteor/meteor';
import { Oidc } from 'meteor/oidc';
import { createAccessModuleClient } from 'meteor/nog-access-2';
import { createReadyJwtsModuleClient } from 'meteor/nog-ready-jwts';

import { renderApp } from './main-ui.jsx';
import {
  NsAccess,
  NsReadyJwts,
} from '../imports/namespaces.js';

const serviceNames = ['gitimp', 'gitzib'];
serviceNames.forEach((service) => {
  Oidc.registerClient(service);
  Oidc.registerOidcService(service);
});

const NogAccess = createAccessModuleClient({
  namespace: NsAccess,
  userId: Meteor.userId.bind(Meteor),
});
window.NogAccess = NogAccess;

const NogReadyJwts = createReadyJwtsModuleClient({
  namespace: NsReadyJwts,
  subscriber: Meteor,
});
window.NogReadyJwts = NogReadyJwts;
const {
  subscribeUserTokens,
  subscribeReadyJwts,
  readyJwts,
  callIssueToken,
  callDeleteUserToken,
} = NogReadyJwts;

function currentUser() {
  return Meteor.user();
}

function getUserTokens() {
  return (((Meteor.user() || {}).services || {}).nogfsoiam || {}).jwts || [];
}

const {
  loginWithGitimp,
  loginWithGitzib,
  logout,
} = Meteor;

const appTitle = 'Nog';

Meteor.startup(() => {
  renderApp({
    appTitle,
    currentUser,
    loginWithGitimp,
    loginWithGitzib,
    logout,
    subscribeReadyJwts,
    readyJwts,
    callIssueToken,
    callDeleteUserToken,
    subscribeUserTokens,
    getUserTokens,
  });
});
