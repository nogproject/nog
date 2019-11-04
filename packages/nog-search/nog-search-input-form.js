import { Meteor } from 'meteor/meteor';
import { Template } from 'meteor/templating';
import { _ } from 'meteor/underscore';
import { ReactiveVar } from 'meteor/reactive-var';
import { SimpleSchema } from 'meteor/aldeed:simple-schema';
import { check } from 'meteor/check';
import './nog-search-input-form.html';


function resolveSearchAliases(input) {
  const aliases = input.match(/(\{\{.*?\}\})/g);
  let searchString = input;
  const unknownAliases = [];
  const userId = Meteor.userId();
  if (aliases && userId) {
    for (const n of aliases) {
      const name = n.replace(/{|}/g, '');
      const res = Meteor.users.findOne({
        _id: userId,
      }, {
        fields: { searchAliases: 1 },
      });
      if (res) {
        let aliasString = '';
        const alias = res.searchAliases.find((d) => d.aliasName === name);
        if (alias) {
          aliasString = alias.aliasString;
        } else {
          unknownAliases.push(n);
        }
        searchString = searchString.replace(n, aliasString);
      }
    }
  }
  searchString = searchString.replace(/^(\s*)/, '');
  return {
    searchString,
    unknownAliases,
  };
}


const matchRouter = Match.Where((x) => {
  check(x.getParam, Function);
  return true;
});


Template.nogSearchInputForm.onCreated(function onCreated() {
  const tpl = this;
  const { data } = tpl;
  check(data, {
    router: matchRouter,
    inputFormLabel: String,
    onUpdateInput: Function,
    updateOnEnter: Match.Maybe(Boolean),
    completion: Match.Maybe(Object),
  });

  _.extend(tpl, {
    unknownAliases: new ReactiveVar([]),
    inputString: new ReactiveVar(''),
    completionString: new ReactiveVar(''),
    itemSel: new ReactiveVar(-1),
    chosenItem: new ReactiveVar(null),
    router: data.router,

    setQueryToUrl(inputString) {
      const params = {};
      if (inputString === '') {
        params.q = null;
      } else {
        params.q = inputString;
      }
      this.router.setQueryParams(params);
    },

    getQueryFromUrl() {
      const params = this.router.current().queryParams;
      return params.q || '';
    },

    addToInput(str) {
      const input = tpl.$('.js-search-input').val();
      const parts = input.split(' ');
      parts[parts.length - 1] = str;
      tpl.$('.js-search-input').val(parts.join(' '));
    },
  });

  Template.currentData().onUpdateInput(this.getQueryFromUrl());

  this.autorun(() => {
    new SimpleSchema({
      inputFormLabel: { type: String },
      updateOnEnter: { type: Boolean, optional: true },
      onUpdateInput: { type: Function },
      completion: { type: Object, optional: true },
    }).validate(Template.currentData);
    if (tpl.completionString.get() === '') {
      tpl.itemSel.set(-1);
    }
  });
});


Template.nogSearchInputForm.helpers({
  displayInputWarnings() {
    const tpl = Template.instance();
    const cdat = Template.currentData();
    const unknown = {
      show: false,
      logs: [],
    };
    const unknownAliases = tpl.unknownAliases.get();
    if (unknownAliases.length > 0) {
      unknown.logs.push({
        items: unknownAliases,
        text: unknownAliases.length > 1 ? 'Unknown aliases' : 'Unknown alias',
      });
      unknown.show = true;
    }
    const unknownKeys = cdat.completion ? cdat.completion.unknownKeys : [];
    if (unknownKeys.length > 0) {
      unknown.logs.push({
        items: unknownKeys,
        text: unknownKeys.length > 1 ? 'Unknown keys' : 'Unknown key',
      });
      unknown.show = true;
    }
    return unknown;
  },

  completion() {
    const cdat = Template.currentData();
    const tpl = Template.instance();
    const completionList = cdat.completion ? cdat.completion.list : [];
    return {
      completionList,
      completionString: tpl.completionString.get(),
      onChooseItem(item) {
        tpl.completionString.set('');
        tpl.addToInput(item);
        tpl.$('.js-search-input').focus();
      },
    };
  },

  queryString() {
    const tpl = Template.instance();
    return tpl.getQueryFromUrl();
  },
});

Template.nogSearchInputForm.events({
  'keydown .js-search-input'(event) {
    const tpl = Template.instance();
    if ([38, 40].indexOf(event.which) > -1) {
      const completionItems = tpl.$('.nog-completion-item');
      const nItems = completionItems.length;
      if (nItems === 0) {
        return;
      }
      let idx = tpl.itemSel.get();
      completionItems.eq(idx).removeClass('selected');
      if (event.which === 38) { // keycode 38: arrow up
        if (idx === -1) {
          idx = nItems;
        }
        completionItems.eq(idx).removeClass('selected');
        idx = (idx - 1 + nItems) % nItems;
      }
      if (event.which === 40) { // keycode 40: arrow down
        idx = (idx + 1) % nItems;
      }
      tpl.itemSel.set(idx);
      completionItems.eq(idx).addClass('selected');
      tpl.addToInput(completionItems[idx].id);
    }
  },

  'keyup .js-search-input'(event) {
    event.preventDefault();
    if ([38, 40].indexOf(event.which) > -1) {
      return;
    }
    const tpl = Template.instance();
    const val = tpl.$('.js-search-input').val();
    tpl.inputString.set(val);
    tpl.completionString.set(tpl.inputString.get().split(' ').pop());
    if (this.updateOnEnter && event.which !== 13) {
      return;
    }
    const resolved = resolveSearchAliases(val);
    tpl.unknownAliases.set(resolved.unknownAliases);
    tpl.setQueryToUrl(resolved.searchString);
    this.onUpdateInput(resolved.searchString);
  },
});


Template.searchWarnUnknownInput.onCreated(function onCreated() {
  this.autorun(() => {
    check(Template.currentData(), {
      unknown: {
        show: Boolean,
        logs: [{
          text: String,
          items: [String],
        }],
      },
    });
  });
});


Template.searchAutocomplete.onCreated(function onCreated() {
  const tpl = this;
  _.extend(tpl, {
    completionList: new ReactiveVar([]),
  });

  this.autorun(() => {
    const cdat = Template.currentData();
    check(cdat, {
      completionList: [String],
      completionString: String,
      onChooseItem: Function,
    });
    const substr = cdat.completionString;
    if (substr === '') {
      tpl.completionList.set([]);
      return;
    }
    const list = _.filter(cdat.completionList, function includeStr(str) {
      return str.includes(substr);
    });
    tpl.completionList.set(list);
  });
});

Template.searchAutocomplete.helpers({
  hasList() {
    return Template.instance().completionList.get().length > 0;
  },

  words() {
    const list = Template.instance().completionList.get();
    const substr = Template.currentData().completionString;
    return _.map(list, function replaceStr(str) {
      return {
        renderString: str.replace(
          substr, `<strong>${substr}</strong>`
        ),
        id: str,
      };
    });
  },
});

Template.searchAutocomplete.events({
  'click .js-completion-item'(event) {
    event.preventDefault();
    Template.currentData().onChooseItem(event.currentTarget.id);
  },
});

