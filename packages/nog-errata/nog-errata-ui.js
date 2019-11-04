import { Template } from 'meteor/templating';
import { Meteor } from 'meteor/meteor';
import { _ } from 'meteor/underscore';

import './nog-errata-ui.html';


const eraSpecs = Object.freeze(_.object(
  Meteor.settings.public.errata.map(era => [era.code, era])
));


const fmtSpec = ({ code, description }) => `
[${code}] ${description}
`;

const fmtGeneric = ({ code }) => `
[${code}] This entry has been marked with errata code ${code}.
An errata code usually means that you should take some action to correct it.
Contact an administrator for more information.
`;

function fmtErrata(errata) {
  const msgs = errata.map((era) => {
    const spec = eraSpecs[era.code];
    return spec ? fmtSpec(spec) : fmtGeneric(era);
  });
  return msgs.join('<br />');
}


Template.nogFilesEntryErrata.onRendered(function onRendered() {
  this.$('button').popover();
});

Template.nogFilesEntryErrata.helpers({
  message() {
    return fmtErrata(this.errata);
  },
});


Template.nogFilesObjectViewErrata.helpers({
  message() {
    return fmtErrata(this.errata);
  },
});


Template.nogWorkspaceRepoMasterContentErrata.helpers({
  message() {
    return fmtErrata(this.errata);
  },
});
