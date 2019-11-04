import { Meteor } from 'meteor/meteor';
import { Oidc } from 'meteor/oidc';

import { renderApp } from './main-ui.jsx';

import 'bootstrap/dist/css/bootstrap.css';

Oidc.registerClient('gitimp');
Oidc.registerOidcService('gitimp');
Oidc.registerClient('gitzib');
Oidc.registerOidcService('gitzib');

function currentUser() {
  return Meteor.user();
}

const {
  loginWithGitimp,
  loginWithGitzib,
  logout,
} = Meteor;

renderApp({
  currentUser,
  loginWithGitimp,
  loginWithGitzib,
  logout,
});
