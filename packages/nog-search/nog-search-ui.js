/* eslint no-underscore-dangle: ["error", { "allow": ["_publishHandle"] }] */

import { Meteor } from 'meteor/meteor';
import { Template } from 'meteor/templating';
import { EasySearch } from 'meteor/easy:search';
import { _ } from 'meteor/underscore';
import { ReactiveVar } from 'meteor/reactive-var';
import { ReactiveDict } from 'meteor/reactive-dict';
import { $ } from 'meteor/jquery';
import { NogError } from 'meteor/nog-error';
import { NogSearch } from './nog-search-methods.js';

const { defaultErrorHandler } = NogError;

const easySearchInit = {
  component: new ReactiveDict(),
  isCreated: 'isCreated',
};

EasySearch.NogComponent =
  class NogComponent extends EasySearch.SingleIndexComponent {

    onCreated() {
      super.onCreated();
      easySearchInit.component.set(easySearchInit.isCreated, true);
      _.extend(this, {
        numTotal: new ReactiveVar(0),
        numCurrent: new ReactiveVar(0),
        searching: new ReactiveVar(false),
        empty: new ReactiveVar(true),
      });
      // Resets `searchDefinition` when navigating back from another page.
      this.index.getComponentDict().set('searchDefinition', '');

      this.autorun(() => {
        const sub =
          this.index.getComponentMethods().getCursor()._publishHandle;
        if (sub) {
          if (sub.ready()) {
            this.searching.set(false);
            const noSearchDef = !this.index.getComponentDict()
                .get('searchDefinition');
            this.empty.set(noSearchDef);
            this.numCurrent.set(this.index.getComponentDict()
                .get('currentCount'));
            this.numTotal.set(this.index.getComponentDict().get('count'));
          } else {
            this.searching.set(true);
          }
        }
      });
    }

    isSearching() {
      return this.searching.get();
    }

    inputIsEmpty() {
      return this.empty.get();
    }

    numHitsCurrent() {
      return this.numCurrent.get();
    }

    numHitsTotal() {
      return this.numTotal.get();
    }
};

EasySearch.NogComponent.register('EasySearch.NogComponent');


Template.searchSettings.onCreated(function onCreated() {
  const tpl = this;
  _.extend(this, {
    aliases: new ReactiveDict(),
  });

  this.autorun(() => {
    const userId = Meteor.userId();
    if (!userId) {
      return;
    }
    const doc = Meteor.users.findOne(
      { _id: userId },
      { fields: { searchAliases: 1 } }
    );
    tpl.aliases.clear();
    if (doc && doc.searchAliases) {
      for (const i of doc.searchAliases) {
        tpl.aliases.set(i.aliasName, _.extend({ isEditing: false }, i));
      }
    }
  });
});


Template.searchSettings.helpers({
  aliases() {
    const aliasesDict = Template.instance().aliases.all();
    const aliasesArray = [];
    for (const key of Object.keys(aliasesDict)) {
      aliasesArray.push(aliasesDict[key]);
    }
    return aliasesArray;
  },
});


Template.searchSettings.events({
  'click .js-search-alias-add, keyup .js-search-alias-string'(event) {
    event.preventDefault();
    if (event.type === 'keyup' && event.which !== 13) {
      return;
    }
    const nameIn = $('.js-search-alias-name');
    const stringIn = $('.js-search-alias-string');
    const opts = {
      aliasName: nameIn.val(),
      aliasString: stringIn.val(),
    };
    NogSearch.addSearchAlias.call(opts, (err) => {
      if (err) {
        defaultErrorHandler(err);
      }
    });
    nameIn.val('');
    stringIn.val('');
  },

  'click .js-search-alias-delete'(event) {
    event.preventDefault();
    const aliasName = event.currentTarget.closest('tr').id;
    NogSearch.deleteSearchAlias.call({ aliasName }, (err) => {
      if (err) {
        defaultErrorHandler(err);
      }
    });
  },

  'click .js-search-alias-edit, keyup .editString'(event) {
    event.preventDefault();
    if (event.type === 'keyup' && event.which !== 13) {
      return;
    }
    const tpl = Template.instance();
    const name = event.currentTarget.closest('tr').id;

    // deselect other selected rows
    const aliasesDict = tpl.aliases.all();
    for (const key of Object.keys(aliasesDict)) {
      const a = aliasesDict[key];
      if (a.isEditing && a.aliasName !== name) {
        a.isEditing = false;
        tpl.aliases.set(a.aliasName, a);
      }
    }

    const obj = tpl.aliases.get(name);
    obj.isEditing = !obj.isEditing;
    tpl.aliases.set(name, obj);

    if (!obj.isEditing) {
      const aliasNameNew = $(`#${name}`).find('.editName').text();
      const aliasStringNew = $(`#${name}`).find('.editString').text();
      const opts = {
        aliasNameOld: obj.aliasName,
        aliasStringOld: obj.aliasString,
        aliasNameNew,
        aliasStringNew,
      };
      NogSearch.editSearchAlias.call(opts, (err) => {
        if (err) {
          defaultErrorHandler(err);
        }
      });
    }
  },
});


export { easySearchInit };
