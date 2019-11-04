// `fixedMd` contains properties and items in a serialization format that could
// be stored, for example, in an fso repo.  Nog app would load it from there
// and insert corresponding docs into MongoDB.
const fixedMd = [];

// `fixedSuggestionNamespaces` contains operations that control suggestion
// namespaces.
const fixedSuggestionNamespaces = [];

fixedMd.push(
  // `Quantity` is similar to Wikidata `Quantity (Q29934271)`.  It is define in
  // the package source, because quantity auto-suggestion depends on it.
  {
    type: 'Item',
    id: '$nogSymbolItemId',
    symbol: 'Quantity',
    names: [
      'Quantity',
    ],
    description: (
      'Quantity is the property datatype that indicates that ' +
      'the value should be a number with a unit.'
    ),
  },
);

// The following examples illustrates how to remove properties and items.
fixedMd.push(
  {
    type: 'RemovedItem',
    id: '$nogScopeSymbolItemId',
    mdns: '/sys/md/g/visual',
    symbol: 'Removed Person Example',
    names: [
      'Removed Person Example',
    ],
    description: (
      'Removed Person Example illustrates how to remove an item.'
    ),
    ofType: [
      'Q5',
    ],
  },
  {
    type: 'RemovedProperty',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'keywords_removed_example',
    names: ['Keywords', 'Topics'],
    nameTokens: [ // optional, default is `$tokensFromNames`.
      'keywords',
      'topics',
    ],
    description: (
      'Keywords can be used to associate search terms with any content.'
    ),
    examples: [
      'cellular orientation',
    ],
    suggestValues: 'TypedItem',
    suggestValuesParams: {
      ofType: ['$knownThis'],
    },
    knownValues: [
      'amira',
      'bcpfs',
      'fluoromath',
      'hand',
      'release',
    ],
  },
);
fixedSuggestionNamespaces.push(
  {
    op: 'DisableProperty',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'keywords_removed_example',
    sugns: '/sys/sug/g/visual',
  },
  {
    op: 'DisablePropertyType',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'keywords_removed_example',
    sugns: '/sys/sug/g/visual',
    suggestFromMdnss: [
      '/sys/md/wikidata',
      '/sys/md/g/visual',
    ],
  },
);

export {
  fixedMd,
  fixedSuggestionNamespaces,
};
