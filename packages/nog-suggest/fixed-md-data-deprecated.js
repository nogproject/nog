// eslint-disable-next-line no-unused-vars
const fixedMdDeprecated = [];

// eslint-disable-next-line no-unused-vars
const fixedSuggestionNamespacesDeprecated = [];

fixedMdDeprecated.push(
  {
    type: 'Property',
    // NogScopeSymbolProperty UUID from `mdns` and `symbol`.
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'keywords',
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

  {
    type: 'Property',
    id: '$nogSymbolPropertyId',
    symbol: 'author',
    // No `names` indicates that it is a partial property that is used when
    // suggesting values but not when adding a metadata field.
    suggestValues: 'TypedItem',
    suggestValuesParams: {
      ofType: [
        'Q5',
        '$knownThis',
      ],
    },
    knownValues: [
      'Steffen Prohaska',
      'Uli Homberg',
      'Marc Osterland',
    ],
  },

  {
    type: 'Property',
    id: '$nogSymbolPropertyId',
    symbol: 'doi',
    names: ['DOI', 'Digital Object Identifier'],
    description: (
      'DOI is a character string that is used as a permanent identifier for ' +
      'a digital object, in a format controlled by the International DOI' +
      'Foundation.'
    ),
    examples: [
      'http://dx.doi.org/10.1007/s11440-014-0308-1',
    ],
  },

  {
    type: 'Property',
    id: '$nogSymbolPropertyId',
    symbol: 'opus_url',
    names: ['OPUS URL'],
    description: (
      'OPUS URL points to a ZIB publication in OPUS.'
    ),
    examples: [
      'https://opus4.kobv.de/opus4-zib/frontdoor/index/index/docId/4397',
    ],
  },

  {
    type: 'Property',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'tags',
    names: ['Tags', 'Labels'],
    description: (
      'Tags are strings without space that can be used to search content. ' +
      'To define a tag, simply start using it, similar to a hashtag.'
    ),
    suggestValues: 'TypedItem',
    suggestValuesParams: {
      ofType: ['$knownThis'],
    },
    examples: [
      '1247Bp4',
      'MyProject',
    ],
  },

  {
    type: 'Property',
    id: '$nogSymbolPropertyId',
    symbol: 'topics',
    names: ['Topics'],
    description: (
      'Topics are added by catalog maintainers to group catalog entries.'
    ),
    examples: [
      'publication',
      'repo',
      'video',
    ],
  },

  {
    type: 'Property',
    id: '$nogSymbolPropertyId',
    symbol: 'imaging_date',
    names: [
      'Imaging Date',
      'Date of Imaging',
      'Acquisition Date',
      'Date of Image Acquisition',
    ],
    description: (
      'Imaging Date is the date when image data was acquired. ' +
      'It should be specified in ISO format.'
    ),
    examples: [
      '2015-12-29',
    ],
  },
);

fixedMdDeprecated.push(
  // The Wikidata item 'human (Q5)`.
  {
    type: 'Item',
    id: 'Q5',
    symbol: 'human',
    names: [
      'human',
      'person',
    ],
    description: (
      'An individual human being.'
    ),
  },

  {
    type: 'Item',
    id: '$nogScopeSymbolItemId',
    mdns: '/sys/md/g/visual',
    symbol: 'Steffen Prohaska',
    names: [
      'Steffen Prohaska',
      'spr',
    ],
    description: (
      'Steffen Prohaska is a researcher at ZIB.'
    ),
    ofType: [
      'Q5',
    ],
  },

  {
    type: 'Item',
    id: '$nogScopeSymbolItemId',
    mdns: '/sys/md/g/visual',
    symbol: 'Uli Homberg',
    names: [
      'Uli Homberg',
      'Ulrike Homberg',
      'uho',
    ],
    description: (
      'Uli Homberg is a researcher at ZIB.'
    ),
    ofType: [
      'Q5',
    ],
  },

  {
    type: 'Property',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'specimen_siteid',
    names: ['Specimen Site ID', 'Site ID'],
    description: (
      'Specimen Site ID identifies the site that processed a specimen.'
    ),
    suggestValues: 'TypedItem',
    suggestValuesParams: {
      ofType: ['$knownThis'],
    },
    examples: [
      '2271',
    ],
  },

  {
    type: 'Property',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'specimen_patient',
    names: ['Specimen Patient ID', 'Patient ID'],
    description: (
      'Specimen Patient ID identifies the patient.'
    ),
    suggestValues: 'TypedItem',
    suggestValuesParams: {
      ofType: ['$knownThis'],
    },
    examples: [
      '2488',
    ],
  },
);

fixedSuggestionNamespacesDeprecated.push(
  {
    op: 'EnableProperty',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'keywords',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnablePropertyType',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'keywords',
    sugns: '/sys/sug/g/visual',
    suggestFromMdnss: [
      '/sys/md/wikidata',
      '/sys/md/g/visual',
    ],
  },

  {
    op: 'EnableProperty',
    id: '$nogSymbolPropertyId',
    symbol: 'doi',
    mdns: '/sys/md/nog',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnableProperty',
    id: '$nogSymbolPropertyId',
    symbol: 'opus_url',
    mdns: '/sys/md/nog',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnableProperty',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'tags',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnableProperty',
    id: '$nogSymbolPropertyId',
    symbol: 'topics',
    mdns: '/sys/md/nog',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnableProperty',
    id: '$nogSymbolPropertyId',
    symbol: 'imaging_date',
    mdns: '/sys/md/nog',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnablePropertyType',
    id: '$nogSymbolPropertyId',
    symbol: 'author',
    sugns: '/sys/sug/g/visual',
    suggestFromMdnss: [
      '/sys/md/wikidata',
      '/sys/md/nog',
      '/sys/md/g/visual',
    ],
  },

  {
    op: 'EnableProperty',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'specimen_siteid',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnableProperty',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'specimen_patient',
    sugns: '/sys/sug/g/visual',
  },

  {
    op: 'EnablePropertyType',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'specimen_siteid',
    sugns: '/sys/sug/g/visual',
    suggestFromMdnss: [
      '/sys/md/g/visual',
    ],
  },

  {
    op: 'EnablePropertyType',
    id: '$nogScopeSymbolPropertyId',
    mdns: '/sys/md/g/visual',
    symbol: 'specimen_patient',
    sugns: '/sys/sug/g/visual',
    suggestFromMdnss: [
      '/sys/md/g/visual',
    ],
  },
);

// `width` and related are now in
// `./tests/manual/test-fixed-md-put-path-metadata.sh`.  See `HACKING-fso.md`.
fixedMdDeprecated.push(
  {
    type: 'Property',
    id: 'P2049', // Wikidata UUID from symbol.
    symbol: 'width',
    names: ['Width'],
    nameTokens: ['$tokensFromNames'],
    description: 'width of an object',
    examples: ['10 nm'],
    suggestValues: 'Quantity',
    suggestValuesParams: {
      units: [
        'Q11573', // `m` Wikidata UUID from `id`.
        'Q175821', // `um` Wikidata UUID from `id`.
        'Q178674', // `nm` Wikidata UUID from `id`.
      ],
    },
  },
  // See
  // <https://www.wikidata.org/wiki/Wikidata:Units#Length_(length_(Q36253))>
  {
    type: 'Item',
    id: 'Q11573', // Wikidata UUID from `id`.
    symbol: 'm',
    names: [
      'metre',
      'm',
      'meter',
      'meters',
      'metres',
    ],
    description: 'SI unit of length',
  },
  {
    type: 'Item',
    id: 'Q175821',
    symbol: 'um',
    names: [
      'micrometre',
      'micrometer',
      'µm',
      'micron',
      'um',
    ],
    description: 'one millionth of a metre',
  },
  {
    type: 'Item',
    id: 'Q178674',
    symbol: 'nm',
    names: [
      'nanometre',
      'nm',
      'nanometer',
    ],
    description: 'unit of length',
  },
);
fixedMdDeprecated.push(
  {
    op: 'EnableProperty',
    id: 'P2049', // width
    mdns: '/sys/md/wikidata',
    sugns: '/sys/sug/default',
  },
  {
    op: 'EnablePropertyType',
    id: 'P2049', // width
    sugns: '/sys/sug/default',
    suggestFromMdnss: [
      '/sys/md/wikidata',
    ],
  },
);
