/* global Package */
import { Meteor } from 'meteor/meteor';
import { createDbChecker } from './dbck-server.js';

function createNogOpsServerApi({
  namespace, checkAccess,
}) {
  const nsMeth = namespace.meth;
  function defMeth(name, func) {
    const qualname = `${nsMeth}.${name}`;
    Meteor.methods({ [qualname]: func });
  }

  const dbChecker = createDbChecker({ checkAccess });
  defMeth('dbck', opts => dbChecker.dbck(Meteor.user(), opts));

  return { dbChecker };
}

function defaultAccess() {
  let checkAccess;
  let testAccess;
  const p = Package['nog-access'];
  if (p == null) {
    console.log(
      '[ops] Default access control disabled, since package `nog-access` ' +
      'is not available.',
    );
    checkAccess = () => undefined;
    testAccess = () => true;
  } else {
    console.log('[ops] Using default access from package `nog-access`.');
    ({ checkAccess, testAccess } = p.NogAccess);
  }
  return { checkAccess, testAccess };
}

function createDefaultApi() {
  const access = defaultAccess();
  const namespace = { meth: 'NogOps' };
  return createNogOpsServerApi({
    namespace,
    ...access,
  });
}

const NogOps = createDefaultApi();
export { NogOps, createNogOpsServerApi };
