import { Meteor } from 'meteor/meteor';
import { check } from 'meteor/check';
import { _ } from 'meteor/underscore';


const defs = [];


function defSetting({
  key, val, help, match, logger = console,
}) {
  let settings = Meteor.settings;
  const path = key.split('.');
  const initial = _.initial(path);
  const last = _.last(path);

  defs.push({ key, val, help });

  for (const p of initial) {
    if (!_.has(settings, p)) {
      settings[p] = {};
    }
    settings = settings[p];
  }

  if (!_.has(settings, last)) {
    settings[last] = val;
    logger.log(
      '[nog-settings] Using default ' +
      `\`settings.${key}=${JSON.stringify(val)}\`.`
    );
  }

  try {
    check(settings[last], match);
  } catch (err) {
    logger.error(
      `[nog-settings] Invalid setting ` +
      `'${key}=${JSON.stringify(settings[last])}': ` +
      `${help} ${err}`
    );
    throw err;
  }
}


// Trim at most one newline on each end of the help text, so that it can be
// written as a multi-line template string without escaping the starting and
// ending newlines.  But don't strip all the whitespace to allow help texts
// that end with an empty line, which can be useful to separate a indented
// code block from the `default: ...` line.

function trimHelp(str) {
  return str.replace(/^\n/, '').replace(/\n$/, '');
}

const fmtDef = ({ key, val, help }) => `\
\`settings.${key}\`:
${trimHelp(help)}
default: ${JSON.stringify(val, null, 2)}
`;

function settingsUsage() {
  return _.sortBy(defs, d => d.key).map(fmtDef).join('\n');
}


export { defSetting, settingsUsage };
