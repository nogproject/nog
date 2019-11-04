import { Template } from 'meteor/templating';
import { Meteor } from 'meteor/meteor';
import { NogAccess } from 'meteor/nog-access';

// Parameterize route names, so that all links point to V1 routes.  See
// `../imports/ui-v2/nog-app-v2.js` for details.
const routes = {
};

Template.layout.helpers({
  optTestingUsers() {
    return Meteor.settings.public.optTestingUsers;
  },

  optHideDisclaimer() {
    return Meteor.settings.public.optHideDisclaimer;
  },

  globals() {
    return {
      router: window.FlowRouter,
      routes,
      nogCatalog: window.NogCatalog,
      nogContent: window.NogContent,
    };
  },
});
