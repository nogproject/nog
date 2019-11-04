// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.
import './package-peer-versions.js';
import { createFsoCatalogPlugin } from './fso-catalog-plugin.js';

export {
  createFsoCatalogPlugin,
};
