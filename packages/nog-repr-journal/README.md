# Journal Representation

This package contains a representation for journal logs in nog.
The entries are shown with metadata and rendered note.

## Function

The package is activated in trees with the meta object `journal: {}`.
Their subtrees are treated as entries. Name, metadata and note are 
displayed for each entry.

## Requirements

In order to work properly, the following requirements have to be
fulfilled additional to the tree structure mentioned above:

 - Metadata has to be stored in `protocol.props` in the entry's
   metadata.
 - The note has to be an object named `note.md`.
