import { check, Match } from 'meteor/check';
import { Suggest } from './autosuggest.js';
import {
  uuidNogKnownClassItem,
  uuidNogPropertySymbolItem,
  uuidNogScopeSymbolItem,
  uuidNogScopeSymbolProperty,
  uuidNogSymbolItem,
  uuidNogSymbolProperty,
  uuidWikidata,
} from './uuid.js';
import {
  tokenizeAcceptedWords,
} from './known-values.js';

function log(msg, ...args) {
  console.log(`[suggest] ${msg}`, ...args);
}

const SuggestionNamespaceOp = {
  EnableProperty: 'EnableProperty',
  DisableProperty: 'DisableProperty',
  EnablePropertyType: 'EnablePropertyType',
  DisablePropertyType: 'DisablePropertyType',
};

// `isWikidataId(id)` returns true if `id` is a Wikidata IDs.  Wikidata IDs are
// `Q<number>` or `P<number>` with a length limit to ensure that
// base64url-encoded UUIDs are not accidentally matched.
function isWikidataId(id) {
  return !!id.match(/^(Q|P)[1-9][0-9]{0,11}$/);
}

function isPropertyData(dat) {
  return dat.type === 'Property';
}

function isRemovedPropertyData(dat) {
  return dat.type === 'RemovedProperty';
}

function isItemData(dat) {
  return dat.type === 'Item';
}

function isRemovedItemData(dat) {
  return dat.type === 'RemovedItem';
}

const matchFixedMdPropertyId = Match.Where((x) => {
  check(x, String);
  return (
    x === '$nogScopeSymbolPropertyId' ||
    x === '$nogSymbolPropertyId' ||
    x.startsWith('P')
  );
});

const matchFixedMdXProperty = Match.Where((x) => {
  check(x, Object);
  if (!(
    x.type === 'Property' ||
    x.type === 'RemovedProperty'
  )) {
    return false;
  }
  check(x, {
    type: String,
    id: matchFixedMdPropertyId,
    mdns: Match.Optional(String),
    symbol: String,
    names: [String],
    nameTokens: Match.Optional([String]),
    description: String,
    examples: [String],
  });
  return true;
});

const matchSuggestEnum = Match.Where((x) => {
  check(x, String);
  return (
    x === 'TypedItem' ||
    x === 'Quantity'
  );
});

const matchFixedMdXPropertyType = Match.Where((x) => {
  check(x, Object);
  if (!(
    x.type === 'Property' ||
    x.type === 'RemovedProperty'
  )) {
    return false;
  }
  check(x, {
    type: String,
    id: matchFixedMdPropertyId,
    mdns: Match.Optional(String),
    symbol: String,
    suggestValues: matchSuggestEnum,
    suggestValuesParams: {
      ofType: Match.Optional([String]),
      units: Match.Optional([String]),
    },
    knownValues: Match.Optional([String]),
  });
  return true;
});

const matchFixedMdXPropertyFull = Match.Where((x) => {
  check(x, Object);
  if (!(
    x.type === 'Property' ||
    x.type === 'RemovedProperty'
  )) {
    return false;
  }
  check(x, {
    type: String,
    id: matchFixedMdPropertyId,
    mdns: Match.Optional(String),
    symbol: String,
    names: [String],
    nameTokens: Match.Optional([String]),
    description: String,
    examples: [String],
    suggestValues: matchSuggestEnum,
    suggestValuesParams: {
      ofType: Match.Optional([String]),
      units: Match.Optional([String]),
    },
    knownValues: Match.Optional([String]),
  });
  return true;
});

const matchFixedMdXPropertyLoose = Match.Where((x) => {
  check(x, Match.OneOf(
    matchFixedMdXProperty,
    matchFixedMdXPropertyType,
    matchFixedMdXPropertyFull,
  ));
  return true;
});

const matchFixedMdItemId = Match.Where((x) => {
  check(x, String);
  return (
    x === '$nogSymbolItemId' ||
    x === '$nogScopeSymbolItemId' ||
    x.startsWith('Q')
  );
});

const matchFixedMdXItem = Match.Where((x) => {
  check(x, Object);
  if (!(
    x.type === 'Item' ||
    x.type === 'RemovedItem'
  )) {
    return false;
  }
  check(x, {
    type: String,
    id: matchFixedMdItemId,
    mdns: Match.Optional(String),
    symbol: String,
    names: [String],
    description: String,
    ofType: Match.Optional([String]),
  });
  return true;
});

const matchOneFixedMd = Match.Where((x) => {
  check(x, Match.OneOf(
    matchFixedMdXPropertyLoose,
    matchFixedMdXItem,
  ));
  return true;
});

const matchFixedMd = [matchOneFixedMd];

// `uuidItemId()` returns a base64url encoded item UUID, converting a Wikidata
// ID if necessary.
function uuidItemId(id) {
  if (isWikidataId(id)) {
    if (!id.startsWith('Q')) {
      throw new Error('invalid Wikidata item id');
    }
    return uuidWikidata(id);
  }
  return id;
}

function mdNamespaceForData(dat) {
  if (dat.mdns) {
    return dat.mdns;
  }
  if (isWikidataId(dat.id)) {
    return '/sys/md/wikidata';
  }
  return '/sys/md/nog';
}

// `uuidForData(data)` returns a UUID for `data`, auto-detecting the right UUID
// namespace by inspecting `data`.
function uuidForData(dat) {
  if (isWikidataId(dat.id)) {
    return uuidWikidata(dat.id);
  }

  if (dat.id === '$nogScopeSymbolPropertyId') {
    return uuidNogScopeSymbolProperty(dat.mdns, dat.symbol);
  }

  if (dat.id === '$nogScopeSymbolItemId') {
    return uuidNogScopeSymbolItem(dat.mdns, dat.symbol);
  }

  if (dat.id === '$nogSymbolPropertyId') {
    return uuidNogSymbolProperty(dat.symbol);
  }

  if (dat.id === '$nogSymbolItemId') {
    return uuidNogSymbolItem(dat.symbol);
  }

  throw new Error('failed to infer UUID type from data');
}

// `compileNamedTokens(data)` returns search tokens for `data`.  The tokens are
// based on the array `data.nameTokens`.  The special value `$tokensFromNames`
// is expanded to lowercase tokens derived by splitting `data.names`.  The
// special value `$tokensFromSymbol` is expaned to tokens derived from
// `data.symbol`.  The default `[$tokensFromSymbol, $tokensFromNames]` is used
// if `data.nameTokens` is undefined.
function compileNamedTokens(dat) {
  const { nameTokens = ['$tokensFromSymbol', '$tokensFromNames'] } = dat;

  const tokens = new Set();
  nameTokens.forEach((t) => {
    if (t === '$tokensFromSymbol') {
      tokenizeAcceptedWords(dat.symbol).forEach((w) => {
        tokens.add(w);
      });
      return;
    }

    if (t === '$tokensFromNames') {
      dat.names.forEach((n) => {
        tokenizeAcceptedWords(n).forEach((w) => {
          tokens.add(w);
        });
      });
      return;
    }

    tokens.add(t);
  });

  return Array.from(tokens);
}

// `sudoInsertFixedMd()` analyzes the array of fixed metadata `fixedMd` and
// inserts properties, property types, and items into the collections,
// including items that represent known values.  See `./fixed-md-data.js` for
// example data.
//
// See `applyFixedMdFromRepo()` for a non-sudo variant with access checks.
function sudoInsertFixedMd({
  mdProperties, mdPropertyTypes, mdItems, fixedMd,
}) {
  check(fixedMd, matchFixedMd);

  // `insertProperty()` inserts an `mdProperties` property that is used when
  // suggesting new metadata fields.
  function insertProperty(dat) {
    // Missing `names` indicates that `dat` only defines a property type to
    // suggest values.  See `insertPropertyType()`.
    if (!dat.names) {
      return;
    }

    const propertyId = uuidForData(dat);
    const {
      symbol, names, description, examples,
    } = dat;
    const $set = {
      symbol, names, description, examples,
      tokens: compileNamedTokens(dat),
      mdns: mdNamespaceForData(dat),
    };
    if (isWikidataId(dat.id)) {
      $set.wdid = dat.id;
    }
    const res = mdProperties.upsert(propertyId, { $set });
    if (res.insertedId) {
      log(
        'Inserted property.',
        'id', propertyId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
      );
    }
  }

  function removeProperty(dat) {
    const id = uuidForData(dat);
    if (mdProperties.remove(id)) {
      log(
        'Removed property.',
        'id', id, 'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
      );
    }
  }

  // `insertPropertyType()` inserts an `mdPropertyTypes` property type that is
  // used when suggesting metadata values.
  function insertPropertyType(dat) {
    // Missing `suggestValues` indicates that `dat` only defines a property to
    // suggest metadata fields.  See `insertProperty()`.
    if (!dat.suggestValues) {
      return;
    }

    const propertyTypeId = uuidForData(dat);
    const {
      symbol,
      suggestValues: suggest,
      suggestValuesParams: suggestParams,
    } = dat;

    switch (suggest) {
      case Suggest.TypedItem: {
        suggestParams.ofType = suggestParams.ofType.map((id) => {
          if (id === '$knownThis') {
            const knownClassId = uuidNogKnownClassItem(propertyTypeId);
            const res = mdItems.upsert(knownClassId, {
              $set: {
                symbol: `known ${symbol}`,
                names: [
                  `known ${symbol}`,
                ],
                description: (isWikidataId(dat.id)) ? (
                  `Values that have been used with ` +
                  `property "${symbol} (${dat.id})" before.`
                ) : (
                  `Values that have been used with ` +
                  `property "${symbol}" before.`
                ),
                mdns: mdNamespaceForData(dat),
              },
            });
            if (res.insertedId) {
              log(
                'Inserted known class item for property type.',
                'propertyTypeId', propertyTypeId,
                'knownClassId', knownClassId,
                'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
              );
            }
            return knownClassId;
          }

          return uuidItemId(id);
        });
        break;
      }

      case Suggest.Quantity: {
        suggestParams.units = suggestParams.units.map(uuidItemId);
        break;
      }

      default:
        throw new Error('unknown `suggestValues`');
    }

    const res = mdPropertyTypes.upsert(propertyTypeId, {
      $set: {
        symbol,
        suggest,
        suggestParams,
        mdns: mdNamespaceForData(dat),
      },
    });
    if (res.insertedId) {
      log(
        'Inserted property type.',
        'id', propertyTypeId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
      );
    }
  }

  function removePropertyType(dat) {
    const propertyTypeId = uuidForData(dat);
    const knownClassId = uuidNogKnownClassItem(propertyTypeId);
    if (mdItems.remove(knownClassId)) {
      log(
        'Removed known class item for property type.',
        'propertyTypeId', propertyTypeId,
        'knownClassId', knownClassId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
      );
    }
    if (mdPropertyTypes.remove(propertyTypeId)) {
      log(
        'Removed property type.',
        'id', propertyTypeId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
      );
    }
  }

  // `insertKnownValues()` inserts items into `mdItems` that represent known
  // values that are used when suggesting metadata values.
  function insertKnownValues(dat) {
    const { knownValues } = dat;
    if (!knownValues) {
      return;
    }

    const knownClassId = uuidNogKnownClassItem(uuidForData(dat));
    const propertyName = (isWikidataId(dat.id)) ? (
      `${dat.symbol} (${dat.id})`
    ) : (
      dat.symbol
    );
    knownValues.forEach((val) => {
      const knownValId = uuidNogPropertySymbolItem(knownClassId, val);
      const res = mdItems.upsert(knownValId, {
        $set: {
          symbol: val,
          names: [val],
          description: `"${val}" has been used in "${propertyName}" before.`,
          ofType: [knownClassId],
          tokens: tokenizeAcceptedWords(val),
          mdns: mdNamespaceForData(dat),
        },
      });
      if (res.insertedId) {
        log(
          'Inserted known value item.',
          'knownValueId', knownValId,
          'ofType', knownClassId,
          'value', val,
        );
      }
    });
  }

  function removeKnownValues(dat) {
    const { knownValues } = dat;
    if (!knownValues) {
      return;
    }

    const knownClassId = uuidNogKnownClassItem(uuidForData(dat));
    knownValues.forEach((val) => {
      const knownValId = uuidNogPropertySymbolItem(knownClassId, val);
      if (mdItems.remove(knownValId)) {
        log(
          'Removed known value item.',
          'knownValueId', knownValId,
          'ofType', knownClassId,
          'value', val,
        );
      }
    });
  }

  // `insertItem()` inserts an item into `mdItems`.
  function insertItem(dat) {
    const itemId = uuidForData(dat);
    const {
      symbol, names, description,
    } = dat;
    const $set = {
      symbol, names, description,
      tokens: compileNamedTokens(dat),
      mdns: mdNamespaceForData(dat),
    };
    if (isWikidataId(dat.id)) {
      $set.wdid = dat.id;
    }
    if (dat.ofType) {
      $set.ofType = dat.ofType.map(uuidItemId);
    }
    const res = mdItems.upsert(itemId, { $set });
    if (res.insertedId) {
      log(
        'Inserted item.',
        'id', itemId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
      );
    }
  }

  function removeItem(dat) {
    const itemId = uuidForData(dat);
    if (mdItems.remove(itemId)) {
      log(
        'Removed item.',
        'id', itemId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
      );
    }
  }

  fixedMd.forEach((dat) => {
    if (isPropertyData(dat)) {
      insertProperty(dat);
      insertPropertyType(dat);
      insertKnownValues(dat);
    } else if (isRemovedPropertyData(dat)) {
      removeProperty(dat);
      removePropertyType(dat);
      removeKnownValues(dat);
    } else if (isItemData(dat)) {
      insertItem(dat);
    } else if (isRemovedItemData(dat)) {
      removeItem(dat);
    } else {
      throw new Error('unexpected fixedMd data type');
    }
  });
}

const matchFSNsOpXableProperty = Match.Where((x) => {
  check(x, Object);
  if (!(
    x.op === SuggestionNamespaceOp.EnableProperty ||
    x.op === SuggestionNamespaceOp.DisableProperty
  )) {
    return false;
  }
  check(x, {
    op: String,
    id: matchFixedMdPropertyId,
    symbol: Match.Optional(String),
    mdns: String,
    sugns: String,
  });
  return true;
});

const matchFSNsOpXablePropertyType = Match.Where((x) => {
  check(x, Object);
  if (!(
    x.op === SuggestionNamespaceOp.EnablePropertyType ||
    x.op === SuggestionNamespaceOp.DisablePropertyType
  )) {
    return false;
  }
  check(x, {
    op: String,
    id: matchFixedMdPropertyId,
    symbol: Match.Optional(String),
    mdns: Match.Optional(String),
    sugns: String,
    suggestFromMdnss: [String],
  });
  return true;
});

const matchFixedSuggestionNamespaceOp = Match.Where((x) => {
  check(x, Match.OneOf(
    matchFSNsOpXableProperty,
    matchFSNsOpXablePropertyType,
  ));
  return true;
});

const matchFixedSuggestionNamespaceOps = [matchFixedSuggestionNamespaceOp];

function sudoApplyFixedSuggestionNamespaces({
  mdProperties, mdPropertyTypes, mdItems, fixedSuggestionNamespaces,
}) {
  check(fixedSuggestionNamespaces, matchFixedSuggestionNamespaceOps);

  function enableProperty(dat) {
    const { mdns, sugns } = dat;

    const id = uuidForData(dat);
    const sel = {
      _id: id,
      mdns,
      sugnss: { $ne: sugns }, // Only update if necessary.
    };
    const $addToSet = { sugnss: sugns };
    if (mdProperties.update(sel, { $addToSet })) {
      log(
        'Enabled property.',
        'id', id,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', mdns, 'sugns', sugns,
      );
    }
  }

  function disableProperty(dat) {
    const { mdns, sugns } = dat;

    const id = uuidForData(dat);
    const sel = {
      _id: id,
      mdns,
      sugnss: { $eq: sugns }, // Only update if necessary.
    };
    const $pull = { sugnss: sugns };
    if (mdProperties.update(sel, { $pull })) {
      log(
        'Disabled property.',
        'id', id,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', mdns, 'sugns', sugns,
      );
    }
  }

  function enablePropertyType(dat) {
    const { suggestFromMdnss, sugns } = dat;

    const pTypeId = uuidForData(dat);
    const selType = {
      _id: pTypeId,
      mdns: { $in: suggestFromMdnss },
    };
    const pType = mdPropertyTypes.findOne(selType);
    if (!pType) {
      log(
        'Did not find property type to enable.',
        'id', pTypeId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
        'suggestFromMdnss', suggestFromMdnss,
      );
      return;
    }

    const $addToSet = { sugnss: sugns };

    // Enable dependencies first.
    switch (pType.suggest) {
      case Suggest.TypedItem: {
        const { ofType } = pType.suggestParams;
        const sel = {
          mdns: { $in: suggestFromMdnss },
          $or: [
            { _id: { $in: ofType } },
            { ofType: { $in: ofType } },
          ],
          sugnss: { $ne: sugns }, // Only update if necessary.
        };

        // XXX Maybe update in multiple steps and verify that the item
        // connectivity is consistent.  All items that represent types, i.e.
        // `_id:{$in:ofType}`, should be included for the types used by the
        // selected items.

        const nUp = mdItems.update(sel, { $addToSet }, { multi: true });
        if (nUp) {
          log(
            'Enabled TypedItem dependencies.',
            'nUpdated', nUp,
            'pTypeId', pTypeId,
            'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
            'suggestFromMdnss', suggestFromMdnss,
          );
        }
        break;
      }

      case Suggest.Quantity: {
        const { units } = pType.suggestParams;
        const sel = {
          _id: { $in: units },
          mdns: { $in: suggestFromMdnss },
          sugnss: { $ne: sugns }, // Only update if necessary.
        };
        const nUp = mdItems.update(sel, { $addToSet }, { multi: true });
        if (nUp) {
          log(
            'Enabled Quantity units.',
            'nUpdated', nUp,
            'pTypeId', pTypeId,
            'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
            'suggestFromMdnss', suggestFromMdnss,
          );
        }
        break;
      }

      default:
        throw new Error('unknown `suggest`');
    }

    Object.assign(selType, {
      sugnss: { $ne: sugns }, // Only update if necessary.
    });
    if (mdPropertyTypes.update(selType, { $addToSet })) {
      log(
        'Enabled property type.',
        'id', pTypeId,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
        'suggestFromMdnss', suggestFromMdnss,
      );
    }
  }

  function disablePropertyType(dat) {
    const { suggestFromMdnss, sugns } = dat;

    const id = uuidForData(dat);
    const selType = {
      _id: id,
      mdns: { $in: suggestFromMdnss },
      sugnss: { $eq: sugns }, // Only update if necessary.
    };
    const $pull = { sugnss: sugns };

    // Do not modify dependencies, because they may be shared with other
    // property types and, therefore, cannot be safely disabled.

    if (mdPropertyTypes.update(selType, { $pull })) {
      log(
        'Disabled property type.',
        'id', id,
        'datId', dat.id, 'symbol', dat.symbol, 'mdns', dat.mdns,
        'suggestFromMdnss', suggestFromMdnss,
      );
    }
  }

  fixedSuggestionNamespaces.forEach((op) => {
    switch (op.op) {
      case SuggestionNamespaceOp.EnableProperty:
        enableProperty(op);
        break;

      case SuggestionNamespaceOp.DisableProperty:
        disableProperty(op);
        break;

      case SuggestionNamespaceOp.EnablePropertyType:
        enablePropertyType(op);
        break;

      case SuggestionNamespaceOp.DisablePropertyType:
        disablePropertyType(op);
        break;

      default:
        throw new Error('unknown `op`');
    }
  });
}

export {
  matchFixedMd,
  matchOneFixedMd,
  matchFixedSuggestionNamespaceOp,
  matchFixedSuggestionNamespaceOps,
  sudoApplyFixedSuggestionNamespaces,
  sudoInsertFixedMd,
  mdNamespaceForData,
};
