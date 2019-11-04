import { Meteor } from 'meteor/meteor';
import { Template } from 'meteor/templating';
import { SimpleSchema } from 'meteor/aldeed:simple-schema';

Template.viewerButtons.onCreated(function onCreated() {
  this.autorun(() => {
    new SimpleSchema({
      fullName: { type: String },
      type: { type: String },
      treePath: { type: String },
      iskindWorkspace: { type: Boolean },
      currentIsFiles: { type: Boolean, optional: true },
      currentIsTechnical: { type: Boolean, optional: true },
      currentIsWorkspace: { type: Boolean, optional: true },
      iskindCatalog: { type: Boolean },
      currentIsCatalog: { type: Boolean, optional: true },
    }).validate(Template.currentData());
  });
});

Template.viewerButtons.helpers({
  optCatalogEnabled() {
    return Meteor.settings.public.optCatalog === 'enabled';
  },
});
