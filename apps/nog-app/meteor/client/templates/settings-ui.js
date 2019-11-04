/* eslint-disable prefer-arrow-callback */

import { Template } from 'meteor/templating';
import { $ } from 'meteor/jquery';

Template.settings.onRendered(function onRendered() {
  if (location.hash !== '') {
    $(`a[href="${location.hash}"]`).tab('show');
  }
});

Template.settings.events({
  'click .js-toggle-tabs'(event) {
    event.preventDefault();
    location.hash = event.currentTarget.hash;
  },
});
