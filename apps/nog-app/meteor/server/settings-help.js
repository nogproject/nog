import { Meteor } from 'meteor/meteor';
import { settingsUsage } from 'meteor/nog-settings';


const fmtUsage = ({ settings }) => `\

========================================

Available settings, from \`defSetting()\`:

${settings}

`;


if (process.env.NOG_SETTINGS_HELP) {
  Meteor.startup(() => {
    console.log(fmtUsage({ settings: settingsUsage() }));
    // Delay exit to ensure that the usage message is flushed.  Without the
    // delay, it was sometimes truncated.
    //
    // The root cause is unclear.  There are GitHub issues about flushing at
    // process exit (google for "node console flush").  The issues claim to be
    // solved.  But maybe there are remaining, related, unknown issues.
    Meteor.setTimeout(() => { process.exit(0); }, 50);
  });
}
