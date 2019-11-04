import { Meteor } from 'meteor/meteor';
import { NogContent } from 'meteor/nog-content';
import { NogAccess } from 'meteor/nog-access';
import { check } from 'meteor/check';

Meteor.publish('toolbar.repo', function toolbarRepo(opts) {
  check(opts, {
    owner: String,
    name: String,
  });

  const aopts = {
    ownerName: opts.owner,
    repoName: opts.name,
  };
  if (!NogAccess.testAccess(this.userId, 'nog-content/get', aopts)) {
    return this.ready();
  }
  return NogContent.repos.find(opts);
});
