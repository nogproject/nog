import { Accounts } from 'meteor/accounts-base';
import { NogAuth } from 'meteor/nog-auth';


// XXX The authenticate hook should be moved to a package to avoid code
// duplication.  See nog-app for duplicate code.

// Nog bots use this special login mechanism to authenticate.  Any valid signed
// request will be accepted.  The convention is to use `GET /ddplogin`.
//
// Connection tokens cannot be disabled, since Meteor immediately closes
// connections without them.

function nogauthv1(req) {
  const { nogauthreq } = req;
  if (nogauthreq == null) {
    return undefined;
  }

  NogAuth.checkRequestAuth(nogauthreq);
  const { user } = nogauthreq.auth;
  if (user == null) {
    return undefined;
  }

  // eslint-disable-next-line no-underscore-dangle
  const userId = user._id;
  return { userId };
}


function registerNogAuthV1() {
  Accounts.registerLoginHandler('nogauthv1', nogauthv1);
}


export { registerNogAuthV1 };
