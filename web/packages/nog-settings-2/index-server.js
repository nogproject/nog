// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.
import './package-peer-versions.js';

import { createSettingsParser } from './parse.js';

export {
  createSettingsParser,
};
