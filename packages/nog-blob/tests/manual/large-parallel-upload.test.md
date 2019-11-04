# Test: Large parallel uploads from browser

- Purpose: Verify that parallel uploads from browsers work as expected.
- Browsers: Chrome, Firefox, Safari.

## Steps

### Setup

Use a local Meteor instance with a local Ceph S3 Docker container as described
in `nog-multi-bucket` devdoc README.

Create a 2 GiB test file:

```bash
head -c 2G /dev/zero >zero-2G
sha1sum zero-2G
```

To reuse the same file after a successful upload, remove the blob:

```
meteor mongo
db.blobs.remove({ _id: '91d50642dd930e9542c39d36f0516d45f4e1af0d' })
```

### Verify multiple parallel uploads

Use a test repo.  Start uploading eight two-gigibyte files in parallel by
repeatedly uploading the same file.

Expected:

 - The upload dialog displays the correct SHA1.
 - All uploads complete successfully.

Known issues:

 - Uploads from Safari may report ETag mismatches.  The mismatches should be
   infrequent, and the uploads should succeed eventually.
