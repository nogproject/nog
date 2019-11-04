# Package `nog-catalog` (Preview)

`nog-catalog` is a framework for collecting entries from other repos in order
to explore them based on their metadata.  Its primary purpose is to organize
microscopy images into image databases for projects and research groups.  We
hope that `nog-catalog` will be more generally useful for similar tasks.

`nog-catalog` has preview status.  The server side implementation contains
access checks and has reasonable test coverage.  The MongoDB operations,
however, should be rate-limited before deploying to production.  The client
side is useful for demonstrating the concept.  But it is completely unpolished
and should not be publicly enabled.

The responsibility in the preview workflow for building a catalog is shared
between two main roles:

 - Imaging users store data that is annotated with a reasonable amount of
   context-specific metadata.
 - Data stewards maintain the data catalogs.

## Data model

`nog-catalog` collects metadata from trees and objects in a group of input
repos and organizes the collected entries in a data collection that refers back
to the original repo paths.  Entries are collected and maintained for the
latest commit on `branches/master`.

A data catalog is managed as a repo.  The repo owner can configure and maintain
the data catalog.  The owner needs read access to content repos for collecting
entries.  The catalog can then be shared by the owner via the circle mechanism
to other users for browsing.  The intended consequence is that other users may
browse metadata of entries to which they have no direct access.  The catalog
owner must keep this in mind when managing sharing.

Metadata on the original entries may be of JSON type `String`, `Number`,
`[String]`, or `[Number]`.  Metadata of other types will be ignored.

We will provide more specific recommendations how to organize metadata in
a continuous process while the amount of data, users, and projects grows.  The
initial suggestion is to use a naming scheme that is inspired by Prometheus's
naming convention, see
<https://prometheus.io/docs/practices/naming/#metric-names>:

 - Metadata keys should have a (single-word) scope prefix.  Examples:
   `imaging_`, `experiment_`, or `processing_`.
 - Metadata keys should use a suffix to specify units, using American spelling
   in plural with prefixes in a single word.  Examples: `_meters`,
   `_micrometers`, `_angstroms`.
 - Related entries should use the same prefix for each unit dimension, for
   example avoid using both angstroms and nanometers or micrometers and
   nanometers in related imaging protocols.
 - Metadata keys should use suffixes to indicate a grouping of vector
   quantities.  Examples: `imaging_pixel_size_x_micrometers`,
   `imaging_pixel_size_y_micrometers`, and
   `imaging_slice_thickness_micrometers`; or `imaging_voxel_size_x_angstroms`,
   `imaging_voxel_size_y_angstroms`, and `imaging_voxel_size_z_angstroms`.

## Metadata entry

Mechanism for efficient metadata entry need to be developed over time.
Initially, data stewards should support imaging users to efficiently attach
metadata.  Over time, imaging users should handle most cases without
assistance.

Current options:

 - Advanced users can use nogpy.

Plans and ideas:

 - Predefined protocols with metadata templates that can be used to annotate
   data during acquisition or right after.
 - Background jobs that automatically extract metadata as stored by acquisition
   devices.
 - Conventions to store metadata in supplementary files in the file system,
   like JSON `image.tiff.meta.json` or YAML `image.tiff.meta.yml` and
   automatically import them with the primary data.
 - Mechanisms to import from other legacy sources such as CSV files with
   metadata for a whole collection of files.

## Maintaining a catalog

Catalogs are currently maintained through Meteor calls from a browser console.
There is no UI yet.  The first step is to configure a catalog.  The second step
is to collect entries.  The collection step needs to be manually repeated to
update a catalog.  Catalogs currently do not automatically update when repos
change.

To configure a catalog, create a file repo and execute:

```javascript
NogCatalog.callConfigureCatalog({ ownerName, repoName, catalogConfig }, console.log);
```

The following example config collect entries from three repos that are
explicitly specified by name.  It uses `$select` to restrict the catalog to
list only entries that have a certain metadata field.  It uses
`$addStaticLabels` to amend the metadata before adding it to the catalog.

```javascript
catalogConfig = {
  preferredMetaKeys: [ 'project', 'origin' ],
  contentRepoConfigs: [
    {
      repoSelector: { owner: 'sprohaska', name: 'emd' },
      pipeline: [
        { $select: {
          'meta.deposition_depositionDate': { $exists: true }
        } },
        { $addStaticLabels: {
          labels: { origin: 'EMDB' }
        } },
      ],
    },
    {
      repoSelector: { owner: 'sprohaska', name: 'idr-102' },
      pipeline: [
        { $select: {
          'meta.Well': { $exists: true }
        } },
        { $addStaticLabels: {
          labels: { origin: 'IDR', project: 'idr-102' },
        } },
      ],
    },
    {
      repoSelector: { owner: 'sprohaska', name: 'idr-597' },
      pipeline: [
        { $select: { 'meta.Well': { $exists: true } } },
        { $addStaticLabels: {
          labels: { origin: 'IDR', project: 'idr-597' },
        } },
      ],
    },
  ]
};
```

To trigger an entry collection to update a catalog, execute:

```javascript
NogCatalog.callUpdateCatalog({ ownerName, repoName }, console.log);
```

Progress is reported to the server console.  An activity log is passed to the
completion callback.

## `NogCatalog.callConfigureCatalog({ ownerName, repoName, catalogConfig })` (client)

`callConfigureCatalog()` sets or updates a catalog config on an existing repo.
The configuration is stored as EJSON in the repo root
`tree.meta.catalog.config`.

`catalogConfig` has the following structure:

```javascript
catalogConfig = {
  preferredMetaKeys: [ <String>, ... ],
  contentRepoConfigs: [
    {
      repoSelector: <MongoSelector>,
      pipeline: [
        <PipelineStep>, ...
      ],
    },
    ...
  ]
};
```

`preferredMetaKeys` is a list of meta key names that are preferred when
creating MongoDB collection indexes.  MongoDB queries on these fields may be
more efficient than queries on meta fields without index.  The number of
collection indexes is limited.  For a large number of different meta key names,
not all meta fields can be supported by a MongoDB collection index.

`contentRepoConfigs` controls entry collection.  `repoSelector` is a MongoDB
selector that selects repos to be considered.  A few examples:

 - `repoSelector: { owner: 'ada' }` selects all repos of one user.
 - `repoSelector: { owner: { $in: ['ada', 'barbara'] }` selects all repos of
   a group of users.
 - `repoSelector: { name: { $regex: '^spindle_[^_]+_2016(-[0-9]{2})?$' } }` selects
   all 2016 spindle repos based on the naming convention
   `<project>_<name>_<year>(-<month>)`.

The latest tree for the selected repos is traversed and all trees and objects
are processed as specified in `pipeline`.  The general syntax for the pipeline
steps is similar to a MongoDB aggregation pipeline, see
<https://docs.mongodb.com/v3.2/core/aggregation-pipeline/>.  The input is the
entry content visited during tree traversal, augmented with the field `path`,
which contains the name path in the repository.  The output of each pipeline
step is passed on to the next step.

Each pipeline step is specified as `{ <operator>: { <args> } }`.  Operators
start with a dollar sign.  The supported operations are:

 - `{ $select: <MinimongoSelector> }`: Entries must match the selector.  Other
   entries are rejected, and pipeline processing stops.
 - `{ $addStaticLabels: { <key>: <value>, ... } }`: The specified static key
   value pairs are added as meta fields `meta.<key> = <value>`, replacing
   fields with the same name if present.

## `NogCatalog.callUpdateCatalog({ ownerName, repoName }, callback)` (client)

`callUpdateCatalog()` initializes or updates a catalog by collecting entries as
specified in the catalog config.  Incremental updates are used for an
unmodified config.  A modified config causes a full re-scan.

An activity log is passed to the completion `callback`.

## Pipeline operator `$select`

The content must match the Minimongo `$select`.  The entry is ignored if it
does not match.

```javascript
pipeline: [
  { $select: { 'meta.Well': { $exists: true } } },
]
```

## Pipeline operator `$updateName`

The pipeline operator `$updateName` sets the name of the catalog entry for
entries that match `$select` based on the content.  The value can be created
using Mustache with `{ $mustache: "<fmt>" }`.  Use triple curly braces to avoid
HTML escaping.  Example:

```javascript
pipeline: [
  { $updateName: {
      $select: { 'meta.std_repo': { $exists: true } },
      $set: { $mustache: "stdrepo {{{meta.std_repo}}}" }
  } },
]
```

## Pipeline operator `$updateMeta`

Similarly to `$updateName`, `$updateMeta` can be used to modify `meta` of
entries that match `$select`.  The modifications can be specified as Minimongo
modifiers `$set`, `$addToSet`, `$unset`, and `$rename`.

Values can be dynamically created using `{ $mustache: <fmt> }` and `{
$splitField: { field, separator, trim } }`.

Examples:

```javascript
pipeline: [
  {
    $updateMeta: {
      $select: { 'meta.std_repo': { $exists: true } },
      $set: { meta_type: 'StdRepo' },
    },
  },
  {
    $updateMeta: {
      $select: { 'meta.year': { $exists: true } },
      $addToSet: { tags: { $mustache: 'Year{{{meta.year}}}' } },
    },
  },
  {
    $updateMeta: {
      $unset: { irrelevant_field: '' },
    },
  },
  {
    $updateMeta: {
      $rename: { author: 'authors' },
    },
  },
  {
    $updateMeta: {
      $select: { 'meta.keywords': { $exists: true } },
      $set: {
        keywords: {
          $splitField: {
            field: 'meta.keywords', separator: ',', trim: true,
          },
        },
      },
    },
  },
],
```

## Pipeline operator `$addStaticLabels`

`$addStaticLabels` is a special case of `$updateMeta`.  It adds static
key-value labels to `meta`.  Example:

```javascript
pipeline: [
  { $select: { 'meta.Well': { $exists: true } } },
  { $addStaticLabels: {
      labels: { origin: 'IDR', project: 'idr-597' },
  } },
]
```
