/* eslint-env browser */

import { _ } from 'meteor/underscore';
import { Template } from 'meteor/templating';
import { ReactiveVar } from 'meteor/reactive-var';
import { ReactiveDict } from 'meteor/reactive-dict';
import { EJSON } from 'meteor/ejson';
import { Counter } from 'meteor/natestrauser:publish-performant-counts';
import { check, Match } from 'meteor/check';
import { NogError } from 'meteor/nog-error';
import './nog-catalog-ui-fso.html';

const {
  defaultErrorHandler,
} = NogError;

const AA_FSO_UPDATE_CATALOG = 'fso/update-catalog';
const AA_FSO_READ_REPO = 'fso/read-repo';

// `theRouter` is set by the toplevel template `nogCatalogDiscoverPresenter`,
// so that it can be used from `nogCatalogTableMetaCell`.
let theRouter = null;


function mayAccess({ nogCatalog, router }) {
  const repoPath = `/${router.getParam('repoPath')}`;
  return nogCatalog.testAccess(AA_FSO_READ_REPO, { path: repoPath });
}

function cleanLogMsgs(logs) {
  const msgs = [];
  for (const l of logs) {
    const msg = l.replace(/(^.*?): /, '');
    msgs.push(msg);
  }
  return msgs;
}

Template.fsoCatalogDiscoverGate.helpers({
  isReady() {
    return mayAccess(this) != null;
  },
  mayAccess() {
    return mayAccess(this);
  },
  layoutArgs() {
    const { data } = Template.instance();
    return {
      nogCatalog: data.nogCatalog,
      router: data.router,
    };
  },
});


const matchRouter = Match.Where((x) => {
  check(x.getParam, Function);
  return true;
});

const matchNogCatalog = Match.Where((x) => {
  check(x.testAccess, Function);
  check(x.volumeRegistry, Object);
  return true;
});

// `router` and `nogCatalog` are attached to the non-reactive
// template instance, since they are static globals and will never trigger any
// changes.
Template.fsoCatalogDiscoverLayout.onCreated(function onCreated() {
  const tpl = this;
  const { data } = tpl;
  check(data, {
    router: matchRouter,
    nogCatalog: matchNogCatalog,
  });
  _.extend(this, {
    router: data.router,
    nogCatalog: data.nogCatalog,
  });
});

Template.fsoCatalogDiscoverLayout.helpers({
  discoverArgs() {
    const tpl = Template.instance();
    return {
      nogCatalog: tpl.nogCatalog,
      router: tpl.router,
    };
  },
});

Template.fsoCatalogDiscoverPresenter.onCreated(function onCreated() {
  const tpl = this;
  const { data } = tpl;
  check(data, {
    router: matchRouter,
    nogCatalog: matchNogCatalog,
  });
  theRouter = data.router;
  _.extend(this, {
    router: data.router,
    nogCatalog: data.nogCatalog,
    filterInput: new ReactiveVar(''),
    volume: new ReactiveVar(null),
    inputString: new ReactiveVar(''),
    isCatalog: new ReactiveVar(true),
    logMessages: new ReactiveDict({}),
    completion: new ReactiveDict({}),
    action: new ReactiveVar(''),
    fieldList: [],
    fieldListPrev: [],

    getCatalogPath() {
      return `/${this.router.getParam('repoPath')}`;
    },

    getActive() {
      const { catalogs } = this.nogCatalog;
      const fsoPath = this.getCatalogPath();
      const catalog = catalogs.findOne({ fsoPath });
      return catalog.active;
    },

    callUpdateCatalog() {
      const fsoPath = this.getCatalogPath();
      const opts = {
        repoPath: fsoPath,
      };
      tpl.resetLogEntries();
      tpl.action.set('Updating ...');
      tpl.nogCatalog.callUpdateCatalogFso(opts, (err, res) => {
        tpl.action.set('');
        if (err) {
          defaultErrorHandler(err);
          return;
        }
        if (res) {
          tpl.updateLogEntries(res.messages);
        }
      });
    },

    convertFilterInput() {
      const active = tpl.getActive();
      const parts = tpl.filterInput.get().split(' ');
      const newParts = [];
      const unknownKeys = [];
      for (const p of parts) {
        const pair = p.split(':');
        const idx = _.indexOf(active.metaKeys, pair[0]);
        if (pair.length === 2 && idx !== -1) {
          const alias = `m${idx}`;
          newParts.push(p.replace(pair[0], alias));
        } else {
          newParts.push(p);
          if (pair.length === 2 && idx === -1) {
            unknownKeys.push(pair[0]);
          }
          if (pair.length > 2) {
            unknownKeys.push(p);
          }
        }
      }
      return {
        string: newParts.join(' '),
        unknownKeys,
      };
    },

    resetLogEntries() {
      tpl.logMessages.set({});
      tpl.fieldListPrev = tpl.fieldList;
    },

    updateLogEntries(msgs) {
      tpl.updateFieldList();
      tpl.logMessages.set('msgs', cleanLogMsgs(msgs));
      tpl.logMessages.set('diffs', tpl.getFieldsDiff());
      tpl.fieldListPrev = tpl.fieldList;
    },

    updateFieldList() {
      const active = tpl.getActive();
      const counts = active.metaKeyCounts || {};
      const fields = active.metaKeys.map((mk, i) => {
        const code = `m${i}`;
        const count = counts[code];
        return { code, name: mk, count };
      });
      tpl.fieldList = _.sortBy(fields, f => f.name);
    },

    getFieldsDiff() {
      const removed = [];
      const added = [];
      const recount = [];

      tpl.fieldListPrev.forEach(function wasRemoved(elt) {
        const idx = tpl.fieldList.findIndex(x => x.name === elt.name);
        if (idx === -1) {
          removed.push(elt);
          return; // eslint-disable-line no-useless-return
        }
      });

      tpl.fieldList.forEach(function wasAdded(elt) {
        const idx = tpl.fieldListPrev.findIndex(x => x.name === elt.name);
        if (idx === -1) {
          added.push(elt);
          return;
        }
        const eltOld = tpl.fieldListPrev[idx];
        if (eltOld.count !== elt.count) {
          if (eltOld.count === 0) {
            added.push(elt);
            return;
          }
          if (elt.count === 0) {
            removed.push(elt);
            return;
          }
          const e = elt;
          e.oldCount = eltOld.count;
          recount.push(e);
          return; // eslint-disable-line no-useless-return
        }
      });

      return { added, removed, recount };
    },
  });

  tpl.completion.set('list', []);
  tpl.completion.set('unknownKeys', []);

  this.autorun(() => {
    if (tpl.filterInput.get() !== tpl.inputString.get()) {
      tpl.filterInput.set(tpl.inputString.get());
    }

    const {
      volumeRegistry,
      subscribeCatalogFso,
      subscribeCatalogHitCountFso,
      subscribeCatalogVolumeFso,
    } = tpl.nogCatalog;

    const fsoPath = this.getCatalogPath();
    const sub = subscribeCatalogFso(this, { repoPath: fsoPath });
    if (!sub.ready()) {
      return;
    }

    const { catalogs } = this.nogCatalog;
    const catalog = catalogs.findOne({ fsoPath });
    const isCatalog = !!catalog;
    tpl.isCatalog.set(isCatalog);
    if (!isCatalog) {
      return;
    }

    const filter = tpl.convertFilterInput();
    const active = tpl.getActive();
    const volumeName = active.volumes[0].name;
    tpl.volume.set(volumeRegistry.getCollection(active, volumeName));
    subscribeCatalogHitCountFso(this, {
      repoPath: fsoPath,
      volumeName,
      filter: filter.string,
    });
    subscribeCatalogVolumeFso(this, {
      repoPath: fsoPath,
      volumeName,
      filter: filter.string,
    });
    tpl.completion.set('list', _.sortBy(active.metaKeys, key => key));
    tpl.completion.set('unknownKeys', filter.unknownKeys);
  });
});

Template.fsoCatalogDiscoverPresenter.helpers({
  isReady() {
    return Template.instance().subscriptionsReady();
  },

  title() {
    const tpl = Template.instance();
    return tpl.getCatalogPath();
  },

  searchInputArgs() {
    const tpl = Template.instance();
    return {
      inputFormLabel: 'Filter',
      updateOnEnter: true,
      onUpdateInput(str) {
        tpl.inputString.set(str);
      },
      completion: tpl.completion.all(),
      router: tpl.router,
    };
  },

  availableFields() {
    const tpl = Template.instance();
    tpl.updateFieldList();
    return tpl.fieldList.filter(e => e.count > 0);
  },

  hitCount() {
    const tpl = Template.instance();
    const active = tpl.getActive();

    const volumeName = active.volumes[0].name;
    return Counter.get(volumeName);
  },

  tableSettings() {
    const tpl = Template.instance();
    return {
      collection: tpl.volume.get(),
      showFilter: false,
      showColumnToggles: false,
      useFontAwesome: true,
      rowsPerPage: 1000,
      showNavigation: 'never',
      fields: [
        {
          label: '',
          sortable: false,
          cellClass: 'col-md-1',
          tmpl: Template.fsoCatalogTableToolCell,
        },
        {
          key: 'name',
          label: 'Name',
          sortable: false,
          cellClass: 'col-md-3',
        },
        {
          key: 'meta',
          label: 'Details',
          sortable: false,
          cellClass: 'col-md-8',
          tmpl: Template.fsoCatalogTableMetaCell,
        },
      ],
    };
  },

  fieldStatsArgs(field) {
    const tpl = Template.instance();
    const { subscribeCatalogVolumeStatsFso } = tpl.nogCatalog;
    return {
      subscribeCatalogVolumeStatsFso,
      volumeNameObj() {
        return {
          repoPath: tpl.getCatalogPath(),
          volumeName: tpl.volume.get()._name,
        };
      },
      volume() {
        return tpl.volume.get();
      },
      field: field.code,
    };
  },

  isCatalog() {
    return Template.instance().isCatalog.get();
  },

  toolsArgs() {
    const tpl = Template.instance();
    return {
      fsoPath: `/${tpl.router.getParam('repoPath')}`,
      onUpdateCatalog() {
        tpl.callUpdateCatalog();
      },
      logMessages() {
        return tpl.logMessages.all();
      },
      testAccess: tpl.nogCatalog.testAccess,
      isUpdating() {
        return tpl.action.get();
      },
    };
  },
});


Template.fsoCatalogTools.onCreated(function onCreated() {
  const tpl = this;
  const { data } = tpl;
  check(data.testAccess, Function);
  _.extend(this, {
    testAccess: data.testAccess,
  });

  this.autorun(() => {
    const cdat = Template.currentData();
    check(cdat, {
      fsoPath: String,
      onUpdateCatalog: Function,
      logMessages: Function,
      testAccess: Match.Any,
      isUpdating: Function,
    });
  });
});

Template.fsoCatalogTools.helpers({
  mayUpdate() {
    const cdat = Template.currentData();
    const tpl = Template.instance();
    const { fsoPath } = cdat;
    return tpl.testAccess(AA_FSO_UPDATE_CATALOG, { path: fsoPath });
  },

  updateLog() {
    const cdat = Template.currentData();
    if (_.isEmpty(cdat.logMessages())) {
      return [];
    }

    const entries = [];
    for (const m of cdat.logMessages().msgs) {
      entries.push(m);
    }
    for (const [k, v] of Object.entries(cdat.logMessages().diffs)) {
      for (const e of v) {
        const entry = _.omit(e, 'code');
        entries.push(`${k}: ${JSON.stringify(entry)}`);
      }
    }
    return entries;
  },
});

Template.fsoCatalogTools.events({
  'click .js-catalog-update'(event) { // eslint-disable-line object-shorthand
    event.preventDefault();
    const cdat = Template.currentData();
    const tpl = Template.instance();
    const { fsoPath } = cdat;
    if (tpl.testAccess(AA_FSO_UPDATE_CATALOG, { path: fsoPath })) {
      cdat.onUpdateCatalog();
    }
  },
});


Template.fsoCatalogTableMetaCell.helpers({
  title() {
    const data = this;
    const json = EJSON.stringify(data.meta, { indent: true, canonical: true });
    return `${json.replace(/[{"}]/g, '').slice(0, 100)}...`;
  },

  metaAsList() {
    const data = this;
    const keys = _.keys(data.meta);
    keys.sort();
    return keys.map(k => ({ key: k, val: data.meta[k] }));
  },

  // `urls()` returns refpath-type-specific links.  If the refpath type is
  // undefined, it is handled as a traditional Nog repo, which was the old
  // default.
  //
  // XXX Consider refactoring the logic to catalog plugins.
  urls() {
    const data = this;
    return data.refpaths.map((rp) => {
      if (rp.type === 'fsorepo') {
        // DEPRECATED: Drop `fsorepo` branch after design conclusion.
        return {
          path: `${rp.path}`,
          url: theRouter.path('fsoRepoV2', {
            repoName: rp.path.replace(/^\/+/, ''),
          }),
        };
      } else if (rp.type === 'fso') {
        return {
          path: rp.repoPath,
          detail: `, subpath ${rp.treePath}`,
          url: theRouter.path('fsoRepoV2', {
            repoName: rp.repoPath.replace(/^\/+/, ''),
          }),
        };
      } else if (rp.type === 'nog' || !rp.type) {
        return {
          path: `${rp.owner}/${rp.repo}/${rp.path}`,
          url: theRouter.path('files', {
            ownerName: rp.owner,
            repoName: rp.repo,
            treePath: rp.path,
          }),
        };
      }
      // fallback: display path type without link.
      return {
        path: `${rp.type} ${rp.path}`,
        url: '',
      };
    });
  },
});


Template.fsoCatalogTableToolCell.onRendered(function onRendered() {
  const tpl = Template.instance();
  tpl.$('[data-toggle="tooltip"]').tooltip();
});

Template.fsoCatalogTableToolCell.events({
  /* eslint-disable-next-line object-shorthand */
  'click .js-catalog-copyPath'(event, templateInstance) {
    const rp = this.refpaths[0];
    const tempInput = document.createElement('input');
    tempInput.value = rp.repoPath.concat(rp.treePath);
    tempInput.innerHTML = tempInput.textContent;
    document.body.appendChild(tempInput);
    tempInput.select();
    const success = document.execCommand('copy');
    if (success) {
      templateInstance.$(event.currentTarget)
        .attr('data-original-title', 'Copied')
        .tooltip('show')
        .attr('data-original-title', 'Copy path to clipboard');
    }
    document.body.removeChild(tempInput);
  },
});


Template.fsoCatalogFieldStatsView.onCreated(function onCreated() {
  _.extend(this, {
    isVisible: new ReactiveVar(false),
  });
});

Template.fsoCatalogFieldStatsView.helpers({
  isVisible() {
    const tpl = Template.instance();
    return tpl.isVisible.get();
  },
});

Template.fsoCatalogFieldStatsView.events({
  'click .js-show-stats'(event) { // eslint-disable-line object-shorthand
    event.preventDefault();
    const tpl = Template.instance();
    tpl.isVisible.set(!tpl.isVisible.get());
  },
});


Template.fsoCatalogFieldStats.onCreated(function onCreated() {
  const tpl = this;
  const { data } = tpl;
  _.extend(this, {
    limit: new ReactiveVar(5),
  });
  this.autorun(() => {
    const nameObj = data.volumeNameObj();
    data.subscribeCatalogVolumeStatsFso(this, {
      ...nameObj,
      field: data.field,
      limit: tpl.limit.get(),
    });
  });
});


Template.fsoCatalogFieldStats.helpers({
  isReady() {
    return Template.instance().subscriptionsReady();
  },

  topk() {
    const data = this;
    const tpl = Template.instance();
    const { stats } = data.volume();
    const { field } = data;
    return stats.find(
      { field },
      {
        sort: { count: -1 },
        limit: tpl.limit.get(),
      },
    );
  },

  k() {
    const data = this;
    const tpl = Template.instance();
    const { stats } = data.volume();
    const { field } = data;
    return stats.find(
      { field },
      {
        sort: { count: -1 },
        limit: tpl.limit.get(),
      },
    ).count();
  },
});

Template.fsoCatalogFieldStats.events({
  'click .js-load-more'(event) { // eslint-disable-line object-shorthand
    event.preventDefault();
    const tpl = Template.instance();
    tpl.limit.set(tpl.limit.get() * 2);
  },
});
