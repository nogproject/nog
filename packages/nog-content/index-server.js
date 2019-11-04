// The import order is a reminiscence of the old `package.js` mechanism, which
// explicitly added files to populate global API objects:
//
//  - Settings are imported first to register defaults.
//  - `nog-content` is then imported to create the API globals.
//  - The remaining imports populate the API globals.

import './nog-content-settings.js';

import { NogContent, NogContentTest } from './nog-content.coffee';

import './nog-content-caching-server.coffee';
import './nog-content-server.coffee';
import './nog-content-treepath.coffee';
import './nog-content-treepath-server.coffee';
import './nog-content-migrations.coffee';


export { NogContent, NogContentTest };
