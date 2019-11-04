import { Meteor } from 'meteor/meteor';

Meteor.publish(null, function publishAliases() {
  if (this.userId) {
    return Meteor.users.find(
      { _id: this.userId },
      { fields: { searchAliases: 1 } },
    );
  }
  return null;
});
