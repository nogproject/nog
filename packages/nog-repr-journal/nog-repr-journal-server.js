import { Meteor } from 'meteor/meteor';
import { NogContent } from 'meteor/nog-content';
import { check } from 'meteor/check';


Meteor.publish('nogJournalTree', function publish(opts) {
  check(opts, {
    sha1: String,
    ownerName: String,
    repoName: String,
  });

  const store = NogContent.store;
  // Access Control
  store.getTree(this.userId, opts);
  const RAW = { transform: false };
  const isPublished = {
    trees: {},
    objects: {},
  };

  const addTree = (sha1) => {
    tree = NogContent.trees.findOne(sha1, RAW);
    if (tree == null) {
      return;
    }
    if (!isPublished.trees[sha1]) {
      const ref = tree.entries;
      for (const e of ref) {
        if (e.type === 'tree') {
          addTree(e.sha1);
        } else if (e.type === 'object') {
          if (!isPublished.objects[e.sha1]) {
            const obj = NogContent.objects.findOne(e.sha1, RAW);
            this.added('objects', e.sha1, obj);
            isPublished.objects[sha1] = true;
          }
        }
      }
      this.added('trees', sha1, tree);
      isPublished.trees[sha1] = true;
    }
  };
  addTree(opts.sha1);
  return this.ready();
});
