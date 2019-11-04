import { Meteor } from 'meteor/meteor';
import { Template } from 'meteor/templating';
import { EJSON } from 'meteor/ejson';
import { NogContent } from 'meteor/nog-content';
import { NogFiles } from 'meteor/nog-files';
import './nog-repr-journal-ui.html';


Meteor.startup(function onStartup() {
  if (NogFiles != null) {
    return NogFiles.registerEntryRepr({
      icon(entryCtx) {
        if (entryCtx.child.content.meta.journal) {
          return 'nogJournalIcon';
        }
        return null;
      },
      view(treeCtx) {
        if (treeCtx.last.type !== 'tree') {
          return null;
        }
        const { content } = treeCtx.last;
        if (content.meta.journal) {
          return 'nogReprJournal';
        }
        return null;
      },
      treePermissions(treeCtx) {
        const ref = treeCtx.contentPath;
        for (const p of ref) {
          if (p.content.meta.journal) {
            return {
              write: false,
            };
          }
        }
        return null;
      },
    });
  }
  return null;
});


Template.nogReprJournal.onCreated(function onCreated() {
  this.autorun(() => {
    const dat = Template.currentData();
    if (dat.last.type === 'tree') {
      this.subscribe('nogJournalTree', {
        ownerName: dat.repo.owner,
        repoName: dat.repo.name,
        sha1: dat.last.content._id,
      });
    }
  });
});

function processEntries(ctx, tree) {
  const title = tree.name;
  let props = 'No metadata available.';
  let hasNote = false;
  if (tree.meta.protocol) {
    if (tree.meta.protocol.props) {
      props = EJSON.stringify(tree.meta.protocol.props,
        {
          indent: true,
          canonical: true,
        });
    }
  }
  let notectx = null;
  for (const e of tree.entries) {
    if (e.type !== 'object') continue;
    const noteobj = NogContent.objects.findOne(e.sha1);
    if (!noteobj) continue;
    if (noteobj.name === 'note.md') {
      const path = [
        ctx.treePath,
        tree.name,
        noteobj.name,
      ];
      notectx = {
        last: {
          content: noteobj,
        },
        treePath: path.join('/'),
        repo: ctx.repo,
        ref: ctx.ref,
        commitId: ctx.commitId,
        namePath: path,
      };
      hasNote = true;
      break;
    }
  }
  return {
    title,
    props,
    hasNote,
    notectx,
  };
}

Template.nogReprJournal.helpers({
  isReady() {
    return Template.instance().subscriptionsReady();
  },

  journalEntries() {
    const output = [];
    const lastEntries = this.last.content.entries;
    for (const e of lastEntries) {
      if (e.type === 'tree') {
        const tree = NogContent.trees.findOne(e.sha1);
        if (tree) {
          output.push(processEntries(this, tree));
        }
      }
    }
    return output;
  },
});
