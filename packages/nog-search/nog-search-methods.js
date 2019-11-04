import { Meteor } from 'meteor/meteor';
import { ValidatedMethod } from 'meteor/mdg:validated-method';
import { SimpleSchema } from 'meteor/aldeed:simple-schema';
import { NogError } from 'meteor/nog-error';
const {
  ERR_PARAM_INVALID,
  ERR_UPDATE,
  nogthrow,
} = NogError;

const NogSearch = {};


NogSearch.addSearchAlias = new ValidatedMethod({
  name: 'addSearchAlias',

  validate: new SimpleSchema({
    aliasName: { type: String },
    aliasString: { type: String },
  }).validator(),

  run({ aliasName, aliasString }) {
    const userId = Meteor.userId();
    if (!userId) {
      return;
    }

    if (aliasName === '' || aliasString === '') {
      nogthrow(
        ERR_PARAM_INVALID, { reason: 'Alias name or string is empty.' }
      );
    }

    if (Meteor.users.findOne(
      { _id: userId, 'searchAliases.aliasName': aliasName })
    ) {
      nogthrow(ERR_UPDATE, { reason: 'Alias already exists.' });
    }

    const n = Meteor.users.update({
      _id: userId,
    }, {
      $push: { searchAliases: { aliasName, aliasString } },
    });

    if (n !== 1) {
      nogthrow(ERR_UPDATE, { reason: 'Failed to save the alias.' });
    }
  },
});


NogSearch.deleteSearchAlias = new ValidatedMethod({
  name: 'deleteSearchAlias',

  validate: new SimpleSchema({
    aliasName: { type: String },
  }).validator(),

  run({ aliasName }) {
    const userId = Meteor.userId();
    if (!userId) {
      return;
    }

    const n = Meteor.users.update(
      { _id: userId },
      { $pull: { searchAliases: { aliasName } } },
    );

    if (n !== 1) {
      nogthrow(ERR_UPDATE, { reason: 'Failed to delete the alias.' });
    }
  },
});


NogSearch.editSearchAlias = new ValidatedMethod({
  name: 'editSearchAlias',

  validate: new SimpleSchema({
    aliasNameOld: { type: String },
    aliasStringOld: { type: String },
    aliasNameNew: { type: String },
    aliasStringNew: { type: String },
  }).validator(),

  run({ aliasNameOld, aliasNameNew, aliasStringOld, aliasStringNew }) {
    const userId = Meteor.userId();
    if (!userId) {
      return;
    }

    if (aliasNameNew === '' || aliasStringOld === '') {
      nogthrow(
        ERR_PARAM_INVALID, { reason: 'Alias name or string is empty.' }
      );
    }

    if (aliasNameNew !== aliasNameOld &&
        Meteor.users.findOne(
          { _id: userId, 'searchAliases.aliasName': aliasNameNew })
    ) {
      nogthrow(ERR_UPDATE, { reason: 'Alias name already exists.' });
    }

    const n = Meteor.users.update(
      {
        _id: userId,
        'searchAliases.aliasName': aliasNameOld,
        'searchAliases.aliasString': aliasStringOld,
      },
      { $set: {
        'searchAliases.$.aliasName': aliasNameNew,
        'searchAliases.$.aliasString': aliasStringNew,
      } },
    );

    if (n !== 1) {
      nogthrow(ERR_UPDATE, { reason: 'Failed to save changes.' });
    }
  },
});


export { NogSearch };
