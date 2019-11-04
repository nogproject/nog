import { check } from 'meteor/check';
import { Email } from 'meteor/email';
import { Accounts } from 'meteor/accounts-base';
import { Meteor } from 'meteor/meteor';

const { from, adminEmails: admins } = Meteor.settings;

// See <https://docs.meteor.com/api/passwords.html#Accounts-emailTemplates>.
Accounts.emailTemplates.from = from;

function send(opts) {
  check(opts, {
    to: [String],
    subject: String,
    text: String,
  });
  Meteor.defer(() => {
    try {
      Email.send({ from, ...opts });
      console.log(
        `[app]: Sent email '${opts.subject}' to ${opts.to.join(', ')}.`,
      );
    } catch (err) {
      console.error('Failed to send email.', 'err', err);
    }
  });
}

function mailToAdminsNewUser(user) {
  const { _id: uid, username, emails, accountType } = user;
  const subject = `[nog] Confirm account '${username}'`;
  const text = `
A new user has joined Nog:

User ID: ${uid}
Username: ${username}
Email: ${emails[0].address}
Account type: ${accountType}

Please review and confirm the account.
`.trim();

  send({
    to: admins,
    subject,
    text,
  });
}

function mailToAdminsUserAddedSignInService({ user, service }) {
  const { _id: uid, username, accountType } = user;
  const subject = (
    `[nog] User '${username}', new sign-in service '${service}'`
  );
  const text = `
The following user has used a new sign-in service for the first time:

User ID: ${uid}
Username: ${username}
Account type: ${accountType}
New sign-in service: ${service}

FYI.  No further action required.
`.trim();

  send({
    to: admins,
    subject,
    text,
  });
}

export {
  mailToAdminsNewUser,
  mailToAdminsUserAddedSignInService,
};
