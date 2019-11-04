const PubNameReadyJwts = 'readyjwts';
const PubNameUserTokens = 'userTokens';

function makePubName(namespace, basename) {
  return `${namespace.pub}.${basename}`;
}

export {
  PubNameReadyJwts,
  PubNameUserTokens,
  makePubName,
};
