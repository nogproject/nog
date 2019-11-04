import { _ } from 'meteor/underscore';
import { Template } from 'meteor/templating';

import './nog-files-ui.html';


Template.nogFilesUploadModalErrorCounts.helpers({
  haveCounts() {
    return (this.counts.nErrors + this.counts.nWarnings > 0);
  },

  errorTextClass() {
    return (this.counts.nErrors > 0) ? 'text-danger' : 'text-muted';
  },

  warningTextClass() {
    return (this.counts.nWarnings > 0) ? 'text-warning' : 'text-muted';
  },
});


Template.nogFilesUploadModalErrors.helpers({
  details(error) {
    if (error.sanitizedError) {
      return error.sanitizedError.details;
    } else if (_.isString(error.details)) {
      return error.details;
    }
    return null;
  },

  alertClass() {
    if (this.severity === 'warning') {
      return 'alert-warning';
    }
    return 'alert-danger';
  },

  alertSign() {
    if (this.severity === 'warning') {
      return 'exclamation-triangle';
    }
    return 'exclamation-circle';
  },

  id() {
    return this.id || this.severity;
  },
});

Template.nogFilesUploadModalErrors.events({
  'click .js-clear-errors'(event) {
    event.preventDefault();
    Template.currentData().onClear();
  },
});
