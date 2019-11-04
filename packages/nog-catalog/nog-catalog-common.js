function makeCollName(namespace, basename) {
  const ns = namespace.coll;
  if (ns) {
    return `${ns}.${basename}`;
  }
  return basename;
}


function makePubName(namespace, basename) {
  return `${namespace.pub}.${basename}`;
}


export { makeCollName, makePubName };
