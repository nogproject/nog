/* global window */

import { Meteor } from 'meteor/meteor';
import { createAccessModuleClient } from 'meteor/nog-access-2';
import { NsAccess } from '../imports/namespace.js';
import { renderApp } from './main-ui.jsx';

import 'bootstrap/dist/css/bootstrap.css';

const NogAccess = createAccessModuleClient({
  namespace: NsAccess,
  userId: Meteor.userId.bind(Meteor),
});
window.NogAccess = NogAccess;
const { testAccess } = NogAccess;

const password = Meteor.settings.public.tests.passwords.user;
const accounts = {
  user: { username: '__testing__user', password },
  guest: { username: '__testing__guest', password },
  admin: { username: '__testing__admin', password },
};

function loginWithPassword(...args) {
  return Meteor.loginWithPassword(...args);
}

function logout(...args) {
  return Meteor.logout(...args);
}

function currentUser() {
  return Meteor.user();
}

Meteor.startup(() => {
  renderApp({
    accounts,
    loginWithPassword,
    logout,
    currentUser,
    testAccess,
  });
});
