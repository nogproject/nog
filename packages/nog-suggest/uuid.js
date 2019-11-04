import uuid5 from 'uuid/v5';

// `base64urlEncode()` returns a base64url string without padding.
function base64urlEncode(buf) {
  let s = buf.toString('base64');
  s = s.replace(/=*$/, '');
  s = s.replace(/\+/g, '-');
  s = s.replace(/\//g, '_');
  return s;
}

function base64urlDecode(str) {
  let s = str;
  s = s.replace(/_/g, '/');
  s = s.replace(/-/g, '+');
  return Buffer.from(s, 'base64');
}

// `uuid5Buffer()` does the same as `uuid5Base64url()` but returns a `Buffer`.
function uuid5Buffer(name, ns) {
  const buf = Buffer.allocUnsafe(16);
  uuid5(name, ns, buf);
  return buf;
}

// `uuid5Base64url()` returns a SHA1-based name UUID v5, encoded as base64url
// string without padding.  The id is suitable as a document ID with Meteor.
function uuid5Base64url(name, ns) {
  return base64urlEncode(uuid5Buffer(name, ns));
}

const UuidNs = {
  // `Wikidata` is the UUID namespace for Wikidata item `Qx` and property `Px`
  // identifiers.  Use `uuidWikidata()` to generate UUIDs from Wikidata IDs.
  Wikidata: '7782fc94-da1d-4164-b9ed-8aba3c5a370f',

  // `NogSymbolItem` is the UUID namespace for items whose ID is derived from
  // the item symbol.  Use `uuidNogSymbolItem()` to generate UUIDs from
  // symbols.
  //
  // Prefer `NogScopeSymbolItem` for new ids.
  NogSymbolItem: '531372d8-4a39-4ec7-af82-c8c46c535dee',

  // `NogSymbolItem` is the UUID namespace for properties whose ID is derived
  // from the property symbol.  Use `uuidNogSymbolProperty()` to generate UUIDs
  // from symbols.
  //
  // Prefer `NogScopeSymbolProperty` for new ids.
  NogSymbolProperty: 'd6ed13e4-d2dd-4fd5-8555-6706fd71a3ea',

  // `NogScopeSymbolItem` is the UUID namespace for items whose ID is derived
  // from the item symbol within a namespace scope.  The namespace is a path
  // like `/sys/md/g/foo`.  The ID is computed in two steps:
  //
  //     ns = uuid5(namespacePath, NogScopeSymbolItem)
  //     id = uuid5(itemSymbol, ns)
  //
  // Use `uuidNogScopeSymbolItem(scopePath, itemSymbol)` to compute UUIDs.
  NogScopeSymbolItem: '825732f0-f0e9-4cc9-a224-f2bb647b7b70',

  // `NogScopeSymbolProperty` is the UUID namespace for properties whose ID is
  // derived from the property symbol within a namespace scope.  The namespace
  // is a path like `/sys/md/g/foo`.  The ID is computed in two steps:
  //
  //     ns = uuid5(scopePath, NogScopeSymbolProperty)
  //     id = uuid5(propertySymbol, ns)
  //
  // Use `uuidNogScopeSymbolProperty(scopePath, propertySymbol)` to compute
  // UUIDs.
  NogScopeSymbolProperty: '4b8a3af1-0844-4d15-946c-45da05731e5b',

  // `NogKnownClassItem` is the UUID namespace for items that represent the
  // class of known values that have been used with a property.  A UUID that
  // represents the class of known values for a specific property is computed
  // as:
  //
  //     uuidKnownClass = uuid5(propertyUUIDBytes, NogKnownClassItem)
  //
  NogKnownClassItem: 'aef19cb0-a152-4dc5-82e8-59a9438f07ae',

  // `NogPropertySymbolItem` is the UUID namespace for items whose ID is
  // derived from a property and an item symbol.  Such IDs are used for items
  // that represent known values that have been used with a property before,
  // for example known keywords.  The UUID is computed in two steps: a UUID
  // namespace is first derived from the property UUID and then used as the
  // UUID namespace for the item symbol.
  //
  //     ns = uuid5(propertyKnownClassUUIDBytes, NogPropertySymbolItem)
  //     id = uuid5(itemSymbol, ns)
  //
  NogPropertySymbolItem: '24af7428-e76a-4b45-a24e-805a394b94f3',
};

function uuidWikidata(wikidataId) {
  return uuid5Base64url(wikidataId, UuidNs.Wikidata);
}

function uuidNogSymbolItem(symbol) {
  return uuid5Base64url(symbol, UuidNs.NogSymbolItem);
}

function uuidNogSymbolProperty(symbol) {
  return uuid5Base64url(symbol, UuidNs.NogSymbolProperty);
}

function uuidNogScopeSymbolItem(nsPath, symbol) {
  const ns = Array.from(uuid5Buffer(nsPath, UuidNs.NogScopeSymbolItem));
  return uuid5Base64url(symbol, ns);
}

function uuidNogScopeSymbolProperty(nsPath, symbol) {
  const ns = Array.from(uuid5Buffer(nsPath, UuidNs.NogScopeSymbolProperty));
  return uuid5Base64url(symbol, ns);
}

function uuidNogKnownClassItem(property) {
  const propId = Array.from(base64urlDecode(property));
  return uuid5Base64url(propId, UuidNs.NogKnownClassItem);
}

function uuidNogPropertySymbolItem(property, symbol) {
  const propId = Array.from(base64urlDecode(property));
  const propNs = Array.from(uuid5Buffer(propId, UuidNs.NogPropertySymbolItem));
  return uuid5Base64url(symbol, propNs);
}

function checkId(expected, got) {
  if (got !== expected) {
    throw new Error(`ID check failed: expected ${expected}, got ${got}`);
  }
}

// Compute IDs then double check.  Use a layout that works with grep, like:
//
// ```
// git grep -i wellknown.*something
// ```
//
const WellKnownItemId = {
  Quantity: uuidNogSymbolItem('Quantity'),
  ZibMember: uuidNogScopeSymbolItem('/sys/md/g/visual', 'ZIB member'),
};
checkId(WellKnownItemId.Quantity, 'DTCwugLIUOWhsU9IMbxnpg');
checkId(WellKnownItemId.ZibMember, 'LMmzw5IVUz-cVMyekD47AA');

export {
  WellKnownItemId,
  uuidNogKnownClassItem,
  uuidNogPropertySymbolItem,
  uuidNogScopeSymbolItem,
  uuidNogScopeSymbolProperty,
  uuidNogSymbolItem,
  uuidNogSymbolProperty,
  uuidWikidata,
};
