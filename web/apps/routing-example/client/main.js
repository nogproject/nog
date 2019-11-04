import { Meteor } from 'meteor/meteor';
import { Mongo } from 'meteor/mongo';
import { renderApp } from './main-ui.jsx';

// This example app has no test users, roles and access checks for the sake of
// simplicity.  It focusses on routing with React Router.

const fakeTokens = new Mongo.Collection(null);
for (let i = 0; i < 10; i += 1) {
  const id = Math.floor(Math.random() * 10000000000000);
  const date = new Date();
  date.setTime(id);
  fakeTokens.insert({
    id: id.toString(),
    expTime: date.toUTCString(),
  });
}

Meteor.startup(() => {
  renderApp({
    fakeTokens,
  });
});
