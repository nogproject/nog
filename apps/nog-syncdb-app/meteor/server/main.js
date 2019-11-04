import { Meteor } from 'meteor/meteor';
import { Mongo } from 'meteor/mongo';
import { check } from 'meteor/check';

import { syncNogDbForever } from '../imports/nog-syncdb.js';


// These settings should work with `tools/bin/meteor-run-rs-syncdb`.
function localhostDefaultSettings() {
  const mongoRsUrl = 'localhost:28017,localhost:28018,localhost:28019';
  return {
    stateId: 'localhost-nog-to-nogdup',
    optForceFullCopy: false,
    waitBeforeCopy_s: 30,
    src: {
      url: `mongodb://${mongoRsUrl}/nog`,
      dbns: 'nog',
      oplogurl: `mongodb://${mongoRsUrl}/local`,
    },
    dst: {
      url: `mongodb://${mongoRsUrl}/nogdup`,
    },
  };
}

const settingsSyncdb = Meteor.settings.syncdb || localhostDefaultSettings();

const states = new Mongo.Collection('syncdb.states');


function main() {
  check(
    settingsSyncdb,
    {
      stateId: String,
      optForceFullCopy: Boolean,
      waitBeforeCopy_s: Number,
      src: {
        url: String,
        dbns: String,
        oplogurl: String,
      },
      dst: {
        url: String,
      },
    },
  );
  syncNogDbForever({ ...settingsSyncdb, states });
}


Meteor.startup(main);
