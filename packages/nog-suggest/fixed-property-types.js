import {
  uuidNogScopeSymbolProperty,
  uuidNogSymbolProperty,
  uuidWikidata,
} from './uuid.js';

// `fixedPropertyTypes` contains properties that have been defined elsewhere,
// such as in `./fixed-md-data.js`.
//
// XXX It should perhaps be derived from information that is stored in
// collections, so that fixed types can be maintained in external storage, such
// as fso repos, and used when discovering known values from catalogs.
const fixedPropertyTypes = new Map();

function insert(mdNamespace, typ) {
  let nsMap = fixedPropertyTypes.get(mdNamespace);
  if (!nsMap) {
    nsMap = new Map();
    fixedPropertyTypes.set(mdNamespace, nsMap);
  }
  nsMap.set(typ.symbol, typ);
}

function findFixedPropertyType(mdNamespace, symbol) {
  const ns = fixedPropertyTypes.get(mdNamespace);
  if (!ns) {
    return null;
  }
  return ns.get(symbol);
}

// The property is used in namespace `mdnsUse`.  It may be defined in a
// different namespace `mdnsDefOther`.
function defNogScopeSymbolProperty(mdnsUse, symbol, mdnsDefOther) {
  const mdnsDef = mdnsDefOther || mdnsUse;
  insert(mdnsUse, {
    symbol,
    id: uuidNogScopeSymbolProperty(mdnsDef, symbol),
    displayName: symbol,
  });
}

// The property is used in the namespace but defined without namespace.
//
// DEPRECATED: Prefer explicit namespace with NogScopeSymbolProperty.
function defNogSymbolProperty(mdNamespace, symbol) {
  insert(mdNamespace, {
    symbol,
    id: uuidNogSymbolProperty(symbol),
    displayName: symbol,
  });
}

// The property is used in the namespace but defined as Wikidata.
function defWikidataProperty(mdNamespace, symbol, id) {
  insert(mdNamespace, {
    symbol,
    id: uuidWikidata(id),
    displayName: `${symbol} (${id})`,
  });
}

const MdNs = {
  Nog: '/sys/md/nog',
  Visual: '/sys/md/g/visual',
};

defNogScopeSymbolProperty(MdNs.Visual, 'keywords', MdNs.Nog);
defNogScopeSymbolProperty(MdNs.Visual, 'specimen_siteid');
defNogScopeSymbolProperty(MdNs.Visual, 'specimen_patient');
defNogScopeSymbolProperty(MdNs.Visual, 'zib_contributors');

// XXX Leftovers from proof of concept.  Maybe remove.
defNogSymbolProperty(MdNs.Visual, 'author');
defNogScopeSymbolProperty(MdNs.Visual, 'tags');
defWikidataProperty(MdNs.Visual, 'width', 'P2049');

export {
  findFixedPropertyType,
};
