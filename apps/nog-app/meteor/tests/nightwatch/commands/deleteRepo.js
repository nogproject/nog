function deleteRepo(client, username, reponame, time) {
  client
    .url(`http://127.0.0.1:3000/${username}/${reponame}/files`)
    .expect.element('.t-repo-settings').to.be.visible.before(time);
  client
    .click('.t-repo-settings')
    .expect.element('.t-delete-repo').to.be.visible.before(time);

  client.click('.t-delete-repo');
  client.expect.element('.t-delete').to.be.visible.before(time);
  client.expect.element('.t-confirm-repo-name').to.be.visible.before(time);

  client
    .setValue('.t-confirm-repo-name', `${username}/${reponame}`)
    .click('.t-delete');
}

exports.deleteRepo = deleteRepo;
