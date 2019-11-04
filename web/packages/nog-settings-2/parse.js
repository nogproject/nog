import { check } from 'meteor/check';
import * as _ from './underscore.js';

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

function createSettingsParser({ logger = console } = {}) {
  const defs = [];

  function defSetting(def) {
    defs.push(def);
  }

  function parseOne(settings, def) {
    let node = settings;
    const {
      key, val, help, match,
    } = def;
    const path = key.split('.');
    const initial = _.initial(path);
    const last = _.last(path);

    for (const p of initial) {
      if (!_.has(node, p)) {
        node[p] = {};
      }
      node = node[p];
    }

    if (!_.has(node, last)) {
      node[last] = val;
      logger.log(
        '[nog-settings] Using default '
        + `\`settings.${key}=${JSON.stringify(val)}\`.`,
      );
    }

    try {
      check(node[last], match);
    } catch (err) {
      logger.error(
        `[nog-settings] Invalid setting `
        + `'${key}=${JSON.stringify(node[last])}': `
        + `${help} ${err}`,
      );
      throw err;
    }
  }

  function parseSettings(settings) {
    for (const def of defs) {
      parseOne(settings, def);
    }
  }

  function settingsUsage() {
    return _.sortBy(defs, d => d.key).map(fmtDef).join('\n');
  }

  const parser = {
    defSetting,
    parseSettings,
    settingsUsage,
  };
  return parser;
}

export {
  createSettingsParser,
};
