import { Meteor } from 'meteor/meteor';
import { Template } from 'meteor/templating';


Template.announcements.helpers({
  announcements() {
    const announcements = Meteor.settings.public.announcements;
    return announcements.map((a) => ({ text: a }));
  },
});
