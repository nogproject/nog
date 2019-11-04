import { nogthrow } from 'meteor/nog-error-2';

const ERR_EMAIL_EXISTS = {
  errorCode: 'ERR_EMAIL_EXISTS',
  statusCode: 403,
  sanitized: {
    errorCode: 'ERR_EMAIL_EXISTS',
    reason: (
      'The server could not determine a unique email address '
      + 'for the account.  '
      + 'You should contact an administrator.'
    ),
  },
  reason: 'Email address already used.',
};

const ERR_GITLAB_GET_EMAIL = {
  errorCode: 'ERR_GITLAB_GET_EMAIL',
  statusCode: 500,
  sanitized: null,
  reason: 'GitLab responded without `data.email`.',
};

const ERR_MERGE_WELLKNOWN_ACCOUNT = {
  errorCode: 'ERR_MERGE_WELLKNOWN_ACCOUNT',
  statusCode: 500,
  sanitized: {
    errorCode: 'ERR_MERGE_WELLKNOWN_ACCOUNT',
    reason: (
      'Failed to add login service to an existing account.  '
      + 'You should contact an administrator.'
    ),
  },
  reason: 'Failed to update wellknown user.',
};

const ERR_UPDATED_WELLKNOWN_ACCOUNT = {
  errorCode: 'ERR_UPDATED_WELLKNOWN_ACCOUNT',
  statusCode: 409,
  sanitized: 'full',
  reason: 'Added login service to existing account.',
};

export {
  ERR_EMAIL_EXISTS,
  ERR_GITLAB_GET_EMAIL,
  ERR_MERGE_WELLKNOWN_ACCOUNT,
  ERR_UPDATED_WELLKNOWN_ACCOUNT,
  nogthrow,
};
