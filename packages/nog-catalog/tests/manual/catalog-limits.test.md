# Test: Create and update a catalog and assess web-app responsiveness.

Purpose: Find acceptable catalog rate limit settings that prevent the GUI from
becoming unresponsive due to catalog updates.

## Steps

### Setup

The test uses one or more testing repos, filled with entries with metadata
attached.  Usually, do not create such repos in a production deployment.  To
create testing repos, adjust the number of repos, number of trees per repo, and
the number of entries per tree at the top of the `catalog-limits-setup.py`
file. Then run:

```
./catalog-limits-setup.py
```

Create a catalog repo by creating a new file repo `ratelimit-catalog`, opening
a Javascript console in the browser and running:

```
catalogConfig = {
  preferredMetaKeys: [ 'repoName', 'treeName' ],
  contentRepoConfigs: [
    {
      repoSelector: { name: { $regex: '^testCatalogUpdateLimits*' } },
      pipeline: [
        { $select: {
          'meta.imageName': { $exists: true }
        } }
      ],
    }
  ]
};

NogCatalog.callConfigureCatalog({ ownerName:'vincent', repoName:'ratelimit-catalog', catalogConfig }, console.log);

```


### Verify performance

Update the catalog repo by running this in the Javascript console:

```
NogCatalog.callUpdateCatalog({ ownerName: 'vincent', repoName: 'ratelimit-catalog' }, console.log);

```

Try various parts of the UI and verify performance.  You can change the size of
the workspace by modifying the setup script.

Try modifying the values of `catalogUpdateReadRateLimit` and
`catalogUpdateWriteRateLimit` in `settings.json`.

Lower values speed-up the computation of the catalog update, but hamper web-app
responsiveness. Higher values should improve responsiveness.

