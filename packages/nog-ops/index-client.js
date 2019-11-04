import { Meteor } from 'meteor/meteor';


function createNogOpsClientApi({ namespace }) {
  const nsMeth = namespace.meth;
  const call = {};
  function defCall(name) {
    const qualname = `${nsMeth}.${name}`;
    call[name] = (...args) => Meteor.call(qualname, ...args);
  }

  defCall('dbck');

  return { call };
}


function createDefaultApi() {
  const namespace = { meth: 'NogOps' };
  return createNogOpsClientApi({ namespace });
}

const NogOps = createDefaultApi();
export { NogOps, createNogOpsClientApi };
