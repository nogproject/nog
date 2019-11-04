import { Template } from 'meteor/templating';
import { ReactiveVar } from 'meteor/reactive-var';
import { _ } from 'meteor/underscore';
import { check, Match } from 'meteor/check';
import { NogError } from 'meteor/nog-error';
import './nog-repo-toolbar.html';

const {
  defaultErrorHandler,
} = NogError;


const matchRouter = Match.Where((x) => {
  check(x.getParam, Function);
  return true;
});

const matchNogCatalog = Match.Where((x) => {
  check(x.testAccess, Function);
  check(x.volumeRegistry, Object);
  return true;
});

const matchNogContent = Match.Where((x) => {
  check(x.call, Object);
  return true;
});


// `router`, `nogContent`, and `nogCatalog` are attached to the non-reactive
// template instance, since they are static globals and will never trigger any
// changes.
Template.nogRepoTopBarPresenter.onCreated(function onCreated() {
  const tpl = this;
  const { data } = tpl;
  check(data.router, matchRouter);
  check(data.nogCatalog, matchNogCatalog);
  check(data.nogContent, matchNogContent);
  check(data.meteorUser, Function);
  check(data.optShowRepoToolbar, Match.Optional(Boolean));
  _.extend(this, {
    router: data.router,
    nogCatalog: data.nogCatalog,
    nogContent: data.nogContent,
    meteorUser: data.meteorUser,
    optShowRepoToolbar: (
      (data.optShowRepoToolbar == null) ? true : data.optShowRepoToolbar
    ),
    forkedFrom: new ReactiveVar(null),
    forkRepo: new ReactiveVar(false),
    forkAction: new ReactiveVar(''),
    repoDoesNotExists: new ReactiveVar(false),

    viewerParams() {
      let refTreePath = '';
      const routeName = tpl.router.getRouteName();
      if (routeName === 'repoTree') {
        refTreePath = 'master';
      }
      return { routeName, refTreePath };
    },

    callForkRepo() {
      tpl.forkAction.set('Forking...');
      const optsFork = {
        old: {
          ownerName: tpl.router.getParam('ownerName'),
          repoName: tpl.router.getParam('repoName'),
        },
        new: {
          ownerName: tpl.meteorUser().username,
        },
      };
      const { routeName, refTreePath } = tpl.viewerParams();
      tpl.nogContent.call.forkRepo(optsFork, function fork(err, res) {
        if (err) {
          return defaultErrorHandler(err);
        }
        if (res) {
          const params = {
            ownerName: res.owner,
            repoName: res.name,
            refTreePath,
          };
          tpl.router.go(routeName, params);
        }
        tpl.forkAction.set('');
        return res;
      });
    },
  });

  this.autorun(() => {
    check(Template.currentData(), {
      ownerName: String,
      repoName: String,
      viewerInfo: Object,
      namePath: Match.Optional([String]),  // see `nogRepoBreadcrumbs`.
      makeHref: Match.Optional(Function),  // see `nogRepoBreadcrumbs`.
      router: Match.Any,
      nogCatalog: Match.Any,
      nogContent: Match.Any,
      meteorUser: Match.Any,
      optShowRepoToolbar: Match.Optional(Boolean),
    });

    const cdat = Template.currentData();
    const opts = {
      owner: cdat.ownerName,
      name: cdat.repoName,
    };
    const sub = tpl.subscribe('toolbar.repo', opts);
    if (!sub.ready()) {
      return;
    }
    const repo = tpl.nogContent.repos.findOne(opts);
    tpl.repoDoesNotExists.set(!repo);
    if (!repo) {
      return;
    }
    if (repo.forkedFrom) {
      tpl.forkedFrom.set(repo.forkedFrom);
    } else {
      tpl.forkedFrom.set(null);
    }
  });
});

Template.nogRepoTopBarPresenter.helpers({
  repoDoesNotExists() {
    return Template.instance().repoDoesNotExists.get();
  },

  optShowRepoToolbar() {
    return Template.instance().optShowRepoToolbar;
  },

  buttonArgs() {
    const cdat = Template.currentData();
    const fullName = `${cdat.ownerName}/${cdat.repoName}`;
    return {
      fullName,
      type: cdat.viewerInfo.type,
      treePath: cdat.viewerInfo.treePath,
      iskindWorkspace: cdat.viewerInfo.iskindWorkspace,
      iskindCatalog: cdat.viewerInfo.iskindCatalog,
      currentIsCatalog: cdat.viewerInfo.currentIsCatalog,
    };
  },

  breadcrumbsArgs() {
    const cdat = Template.currentData();
    return _.pick(cdat, 'ownerName', 'repoName', 'namePath', 'makeHref');
  },

  toolbarArgs() {
    const tpl = Template.instance();
    return {
      router: tpl.router,
      testAccess: tpl.nogCatalog.testAccess,
      meteorUser: tpl.meteorUser,
      onForkRepo() {
        tpl.callForkRepo();
      },
      forkAction() {
        return tpl.forkAction.get();
      },
    };
  },

  forkedArgs() {
    const tpl = Template.instance();
    return {
      routeName() {
        return tpl.router.getRouteName();
      },
      forkedFrom() {
        return tpl.forkedFrom.get();
      },
    };
  },
});

Template.nogRepoBreadcrumbs.onCreated(function onCreated() {
  this.autorun(() => {
    check(Template.currentData(), {
      ownerName: String,
      repoName: String,

      // `namePath` contains the path parts.  If `namePath` is defined,
      // `makeHref()` must also be provided.
      namePath: Match.Optional([String]),

      // `makeHref({ ownerName, repoName, treePath })` returns hrefs for the
      // breadcrumbs.
      makeHref: Match.Optional(Function),
    });
  });
});

Template.nogRepoBreadcrumbs.helpers({
  rootHref() {
    const cdat = Template.currentData();
    const { ownerName, repoName, makeHref } = cdat;
    if (!makeHref) {
      return null;
    }
    return makeHref({ ownerName, repoName });
  },

  hrefPath() {
    const cdat = Template.currentData();
    const { ownerName, repoName, makeHref, namePath } = cdat;
    if (!makeHref || !namePath) {
      return [];
    }

    const prefix = [];
    const hrefPath = [];
    for (const name of _.initial(namePath)) {
      prefix.push(name);
      hrefPath.push({
        name,
        href: makeHref({ ownerName, repoName, treePath: prefix.join('/') }),
      });
    }

    const last = _.last(namePath);
    if (last) {
      hrefPath.push({ name: last });
    }

    return hrefPath;
  },
});
