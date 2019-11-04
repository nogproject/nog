import { Suggest } from './autosuggest.js';
import {
  WellKnownItemId,
  uuidNogKnownClassItem,
  uuidNogPropertySymbolItem,
  uuidNogScopeSymbolProperty,
} from './uuid.js';
import {
  findFixedPropertyType,
} from './fixed-property-types.js';

function log(msg, ...args) {
  console.log(`[suggest] ${msg}`, ...args);
}

const meaninglessWords = new Set([
  'and',
  'of',
  'or',
  'that',
  'the',
  'this',
]);

function isAcceptedWord(s) {
  return s.length > 1 && !meaninglessWords.has(s);
}

// `tokenizeAcceptedWords(val)` converts `val` into a list of lowercase strings
// that are useful for prefix searches.  The result contains `val` itself and
// words that are created by splitting `val`, excluding words that are useless
// for completion.
function tokenizeAcceptedWords(val) {
  const lower = val.trim().toLowerCase();
  // Exclude first split word, since it is a prefix of the full string.
  const tokens = [lower].concat(lower.split(' ').slice(1));
  return tokens.filter(isAcceptedWord);
}

function sudoCreateKnownPropertyInserter({
  mdPropertyTypes, mdItems, mdNamespace, sugNamespaces,
}) {
  function insertKnownPropertyType({ property }) {
    const { property: symbol, ofTypePredict } = property;

    // Fixed property types have been inserted elsewhere.
    if (findFixedPropertyType(mdNamespace, symbol)) {
      return;
    }

    const propertyId = uuidNogScopeSymbolProperty(mdNamespace, symbol);

    // If there is no fixed property, apply heuristics to determine how to
    // suggest.

    // Suggest quantities if any of the predicted types is Quantity.
    function shouldSuggestQuantity() {
      return ofTypePredict.includes(WellKnownItemId.Quantity);
    }

    if (shouldSuggestQuantity()) {
      const res = mdPropertyTypes.upsert(propertyId, {
        $set: {
          symbol,
          suggest: Suggest.Quantity,
          suggestParams: { units: [] },
          mdns: mdNamespace,
        },
        $addToSet: {
          sugnss: { $each: sugNamespaces },
        },
      });
      if (res.insertedId) {
        log(
          'Inserted Quantity property type.',
          'propertyId', propertyId, 'symbol', symbol, 'mdns', mdNamespace,
        );
      }
      return;
    }

    // `knownX` represents the class of values that have been used with
    // `property`.
    const typeidKnownX = uuidNogKnownClassItem(propertyId);
    const res = mdItems.upsert(typeidKnownX, {
      $set: {
        symbol: `known ${symbol}`,
        names: [
          `known ${symbol}`,
        ],
        description: (
          `Values that have been used with property "${symbol}" before.`
        ),
        mdns: mdNamespace,
      },
      $addToSet: {
        sugnss: { $each: sugNamespaces },
      },
    });
    if (res.insertedId) {
      log(
        'Inserted known class item for property type.',
        'propertyId', propertyId,
        'knownClassId', typeidKnownX,
        'symbol', symbol, 'mdns', mdNamespace,
      );
    }

    const res2 = mdPropertyTypes.upsert(propertyId, {
      $set: {
        symbol,
        suggest: Suggest.TypedItem,
        suggestParams: { ofType: ofTypePredict.concat([typeidKnownX]) },
        mdns: mdNamespace,
      },
      $addToSet: {
        sugnss: { $each: sugNamespaces },
      },
    });
    if (res2.insertedId) {
      log(
        'Inserted property type.',
        'propertyId', propertyId, 'symbol', symbol, 'mdns', mdNamespace,
      );
    }
  }

  const pTypes = new Map();

  // `findPropertyType()` is a cached lookup in `mdPropertyTypes` with
  // additional fields `knownClassId` and `displayName`.
  function findPropertyType({ symbol }) {
    let propertyId;
    let displayName;
    const fixed = findFixedPropertyType(mdNamespace, symbol);
    if (fixed) {
      ({ id: propertyId, displayName } = fixed);
    } else {
      propertyId = uuidNogScopeSymbolProperty(mdNamespace, symbol);
      displayName = symbol;
    }

    const p = pTypes.get(propertyId);
    if (p || p === null) {
      return p;
    }

    const doc = mdPropertyTypes.findOne(propertyId);
    if (!doc) {
      pTypes.set(propertyId, null);
      return null;
    }

    doc.displayName = displayName;
    doc.knownClassId = uuidNogKnownClassItem(propertyId);
    pTypes.set(propertyId, doc);
    return doc;
  }

  function insertKnownPropertyValue({ propertySymbol, value }) {
    const pType = findPropertyType({ symbol: propertySymbol });

    // Silently ignore unknown types.
    if (!pType) {
      return 0;
    }

    // Do not suggest known values for quantities.
    if (pType.suggest === Suggest.Quantity) {
      return 0;
    }

    if (pType.suggest !== Suggest.TypedItem) {
      console.error('Unknown suggest type.');
      return 0;
    }

    // Ignore value if property type does not suggest known values.
    const { knownClassId } = pType;
    if (!pType.suggestParams.ofType.includes(knownClassId)) {
      return 0;
    }

    // Accept strings and numbers.  Ignore other types, since it is unclear how
    // to force them to useful strings.
    let valueString;
    if (typeof value === 'string') {
      valueString = value;
    } else if (typeof value === 'number') {
      valueString = String(value);
    } else {
      return 0;
    }

    // Skip if all tokens are useless.
    const tokens = tokenizeAcceptedWords(valueString);
    if (tokens.length === 0) {
      return 0;
    }

    const { displayName } = pType;
    const knownValId = uuidNogPropertySymbolItem(knownClassId, valueString);
    const res = mdItems.upsert(knownValId, {
      $set: {
        symbol: valueString,
        names: [valueString],
        description: (
          `"${valueString}" has been used in "${displayName}" before.`
        ),
        ofType: [knownClassId],
        tokens,
        mdns: mdNamespace,
      },
      $addToSet: {
        sugnss: { $each: sugNamespaces },
      },
    });
    if (res.insertedId) {
      log(
        'Inserted known value item.',
        'knownValueId', knownValId,
        'value', valueString,
        'ofType', knownClassId,
        'propertySymbol', propertySymbol,
      );
    }

    return 1;
  }

  return {
    insertKnownProperties({ properties }) {
      properties.forEach((property) => {
        insertKnownPropertyType({
          mdPropertyTypes, mdItems, mdNamespace, property, sugNamespaces,
        });
      });
    },

    insertKnownPropertyValues({ propertySymbol, values }) {
      let n = 0;
      values.forEach((value) => {
        n += insertKnownPropertyValue({
          mdItems, mdNamespace, propertySymbol, value, sugNamespaces,
        });
      });
      return n;
    },
  };
}

function looksLikeNumber(val) {
  if (typeof val === 'number') {
    return true;
  }

  if (typeof val !== 'string') {
    return false;
  }

  const first = val.split(' ')[0];
  return !Number.isNaN(Number(first));
}

// `createPropertyTypeLearner()` returns a `learner` that can be used to
// analyze property values and learn which types should be used for
// auto-suggestion.
//
// Repeatedly call `learner.update({ property, values })` to gather stats.
// Finally call `learner.predict({ threshold, minCount })` to get a list of `{
// property, ofTypePredict }` for all `property` keys that were passed to
// `update()`.
//
// `ofTypePredict` is a list of types that should be used for auto-suggestion.
// The list may be empty.  Types are included if the `learner` saw at least
// `minCount` values for the property and the ratio of values that indicated a
// certain type is greater or equal `threshold`.  The `known X` type is never
// included.
//
// The current implementation only predicts `Quantity`.  More types should be
// added as needed.
function createPropertyTypeLearner() {
  const stats = new Map();

  return {
    update({ property, values }) {
      if (!stats.get(property)) {
        stats.set(property, new Map());
      }
      const propStats = stats.get(property);

      function incr(key, delta) {
        const old = propStats.get(key) || 0.0;
        propStats.set(key, old + delta);
      }

      values.forEach((v) => {
        if (looksLikeNumber(v)) {
          incr(WellKnownItemId.Quantity, 1);
        }
      });

      incr('count', values.length);
    },

    predict({ threshold, minCount }) {
      const ret = [];
      for (const [property, propStats] of stats.entries()) {
        const ofTypePredict = [];
        const count = propStats.get('count');
        if (count >= minCount) {
          for (const [ty, tyCount] of propStats.entries()) {
            if (ty !== 'count' && (tyCount / count) >= threshold) {
              ofTypePredict.push(ty);
            }
          }
        }
        ret.push({ property, ofTypePredict });
      }
      return ret;
    },
  };
}

export {
  createPropertyTypeLearner,
  sudoCreateKnownPropertyInserter,
  tokenizeAcceptedWords,
};
