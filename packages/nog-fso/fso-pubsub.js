function makePubName(namespace, basename) {
  return `${namespace.pub}.${basename}`;
}

export {
  makePubName,
};
