import { Meteor } from 'meteor/meteor';

import { registerNogAuthV1 } from './accounts.js';

Meteor.startup(() => {
  registerNogAuthV1();
});
