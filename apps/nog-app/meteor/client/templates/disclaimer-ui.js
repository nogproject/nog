import { Meteor } from 'meteor/meteor';
import { Template } from 'meteor/templating';


const defaultMsg = (
  `This is preview software.  We will not deliberately delete your data.  But
  we run without backup.  So please keep a copy of your data elsewhere.`
);

Template.disclaimer.helpers({
  text() {
    const pub = Meteor.settings.public;
    if (pub == null || pub.disclaimer == null || pub.disclaimer === '') {
      return defaultMsg;
    }
    return pub.disclaimer;
  },
});
