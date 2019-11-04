# `NogTree` is the global API object for the tree-ui-related functions.
NogTree = {}


# Registry for repr templates.
#
# Specs are added to the front of the specs list, so that later specs win.
# This can be used to override repr specs from the main app, because it is
# initialized after the packages.

entryReprSpecs = []

NogTree.registerEntryRepr = (spec) -> entryReprSpecs.unshift spec

NogTree.selectEntryRepr = (content) ->
  for r in entryReprSpecs
    if (t = r.selector(content))?
      return t
  return null
