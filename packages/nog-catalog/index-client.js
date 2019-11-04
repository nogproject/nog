/* global Package */
import { Meteor } from 'meteor/meteor';
import { createCatalogClientModule } from './nog-catalog-main-client.js';
import { namespace } from './default-namespace.js';
import './nog-catalog-ui.js';
import './nog-catalog-ui-fso.js';


function defaultAccess() {
  const pkg = Package['nog-access'];
  if (pkg) {
    const { NogAccess } = pkg;
    return {
      testAccess: NogAccess.testAccess,
    };
  }
  return {
    testAccess() {
      return true;
    },
  };
}


const NogCatalog = (
  Meteor.settings.public.optCatalog === 'disabled' ?
    null :
    createCatalogClientModule({ namespace, ...defaultAccess() })
);


export { NogCatalog, createCatalogClientModule };
