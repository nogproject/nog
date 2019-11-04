import { HTTP } from 'meteor/http';

import { nogthrow, ERR_GITLAB_GET_EMAIL } from './errors.js';

// `gitlabGetEmailAddress({url, token})` returns the email address for the
// GitLab user who owns `token`.
//
// See <https://docs.gitlab.com/ce/api/users.html#user>,
// <https://docs.gitlab.com/ce/api/oauth2.html#gitlab-as-an-oauth2-provider>,
// <https://docs.gitlab.com/ce/api/oauth2.html#access-gitlab-api-with-access-token>.
function gitlabGetEmailAddress({ url, token }) {
  const userApi = `${url}/api/v4/user`;
  const headers = { Authorization: `Bearer ${token}` };
  const o = HTTP.get(userApi, { headers });
  if (!o.data || !o.data.email) {
    nogthrow(ERR_GITLAB_GET_EMAIL, {
      reason: 'Failed to retrieve GitLab account email.',
    });
  }
  return o.data.email;
}

export {
  gitlabGetEmailAddress,
};
