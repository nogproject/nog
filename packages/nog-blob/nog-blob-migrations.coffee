{
  ERR_MIGRATION
  nogthrow
} = NogError

NogBlob.migrations = {}

# See commit 'nog-blob: Fix concurrent uploads of same blob' for schema change.
NogBlob.migrations.removeUploadInfoFromBlobs = ->
  oldFields = [
      's3UploadId', 's3PartParams', 's3LocalName', 's3Heartbeat', 's3Parts',
      'issues'
    ]
  sel = {$or: []}
  mod = {$unset: {}}
  for f in oldFields
    s = {}
    s[f] = {$exists: true}
    sel.$or.push s
    mod.$unset[f] = ''
  nErr = 0
  NogBlob.blobs.find(sel).map (blob) ->
    try
      NogBlob.blobs.update blob._id, mod
    catch err
      console.error '
          removeUploadInfoFromBlobs(): Failed to remove old fields:
        ', blob, err
      nErr++
  if nErr
    msg = "removeUploadInfoFromBlobs() failed to migrate #{nErr} documents."
    console.error msg
    nogthrow ERR_MIGRATION, {details: msg}
