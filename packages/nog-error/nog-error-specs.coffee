specs = [

  {
    errorCode: 'ERR_MIGRATION'
    statusCode: 500
    sanitized: null
    reason: 'A database migration failed.'
  }

  {
    errorCode: 'ERR_UNIMPLEMENTED'
    statusCode: 500
    sanitized: null
    reason: 'The operation is not implemented.'
  }

  # Malformed as in XML structure validation: the format of a value is invalid.
  {
    errorCode: 'ERR_PARAM_MALFORMED'
    statusCode: 422
    sanitized: 'full'
    reason: 'A parameter was malformed.'
  }

  # Invalid comes after malformed: the basic structure is ok, but the value is
  # semantically invalid, such as out-of-range.
  {
    errorCode: 'ERR_PARAM_INVALID'
    statusCode: 422
    sanitized: 'full'
    reason: 'A parameter was semantically invalid.'
  }

  {
    errorCode: 'ERR_S3_CREATE_MULTIPART'
    statusCode: 502
    sanitized: null
    reason: 'Failed to create S3 multipart upload.'
    details: (ctx) -> "
        Failed to call createMultipartUpload with S3 bucket `#{ctx.s3Bucket}`,
        and object key `#{ctx.s3ObjectKey}`.
      "
    contextPattern:
      s3Bucket: String
      s3ObjectKey: String
  }
  {
    errorCode: 'ERR_S3_COMPLETE_MULTIPART'
    statusCode: 502
    sanitized: null
    reason: 'Failed to complete S3 multipart upload.'
    details: (ctx) -> "
        Failed to call completeMultipartUpload with S3 bucket `#{ctx.s3Bucket}`,
        and object key `#{ctx.s3ObjectKey}`.
      "
    contextPattern:
      s3Bucket: String
      s3ObjectKey: String
      s3UploadId: String
  }
  {
    errorCode: 'ERR_S3_ABORT_MULTIPART'
    statusCode: 502
    sanitized: null
    reason: 'Failed to abort S3 multipart upload.'
    details: (ctx) -> "
        Failed to call abortMultipartUpload with S3 bucket `#{ctx.s3Bucket}`,
        object key '#{ctx.s3ObjectKey}', and upload id `#{ctx.s3UploadId}`.
      "
    contextPattern:
      s3Bucket: String
      s3ObjectKey: String
      s3UploadId: String
  }

  {
    errorCode: 'ERR_BLOB_NOT_FOUND'
    statusCode: 404
    sanitized: 'full'
    reason: (ctx) -> "The requested blob '#{ctx.blob}' could not be found."
    details: null
    contextPattern:
      blob: String
  }
  {
    errorCode: 'ERR_BLOB_UPLOAD_START'
    statusCode: 502
    sanitized: null
    reason: 'Failed to start upload.'
    details: (ctx) -> "
        The server reported an error when calling 'startMultipartUpload' for
        file `#{ctx.fileName}`, size #{ctx.fileSize}, sha1 `#{ctx.sha1}`.
      "
    contextPattern:
      fileName: String
      fileSize: Number
      sha1: String
  }
  {
    errorCode: 'ERR_BLOB_UPLOAD'
    statusCode: 500
    sanitized: 'full'
    reason: 'Blob upload failed.'
  }
  {
    errorCode: 'ERR_BLOB_UPLOAD_EXISTS'
    statusCode: 409
    sanitized: 'full'
    reason: (ctx) -> "
        The blob '#{ctx.sha1}' already exists and cannot be uploaded again.
        You may continue assuming that the blob is available as if the
        upload succeeded.
      "
    contextPattern:
      sha1: String
  }
  {
    errorCode: 'ERR_BLOB_CONFLICT'
    statusCode: 409
    sanitized: 'full'
    reason: (ctx) -> "
        The blob '#{ctx.sha1}' already exists, but with a different size.
        You should probably contact a system administrator.
      "
    contextPattern:
      sha1: String
  }
  {
    errorCode: 'ERR_BLOB_UPLOAD_WARN'
    statusCode: 500
    sanitized: 'full'
    reason: 'Problem with blob upload that may be resolved later.'
  }
  {
    errorCode: 'ERR_BLOB_COMPUTE_SHA1'
    statusCode: 500
    sanitized: 'full'
    reason: 'Problem computing sha1.'
  }
  {
    errorCode: 'ERR_BLOB_COMPUTE_MD5'
    statusCode: 500
    sanitized: 'full'
    reason: 'Problem computing MD5.'
  }
  {
    errorCode: 'ERR_BLOB_ABORT_PENDING'
    statusCode: 502
    sanitized: null
    reason: "Failed to abort pending upload after timeout."
    contextPattern:
      sha1: String
  }
  {
    errorCode: 'ERR_DB'
    statusCode: 500
    sanitized: null
    reason: "A database operation unexpectedly failed."
  }
  {
    errorCode: 'ERR_LIMIT'
    statusCode: 413
    sanitized: 'full'
    reason: "The request is larger than a limit."
  }
  {
    errorCode: 'ERR_LIMIT_S3_OBJECT_SIZE'
    statusCode: 413
    sanitized: 'full'
    reason: (ctx) -> "
        The upload size (#{ctx.size} Bytes) is greater than the maximum
        size supported by S3 (#{ctx.maxSize} Bytes).
      "
    contextPattern:
      size: Number
      maxSize: Number
  }
  {
    errorCode: 'ERR_UPLOADID_UNKNOWN'
    statusCode: 404
    sanitized: 'full'
    reason: 'The upload id is unknown.'
  }
  {
    errorCode: 'ERR_UPLOAD_COMPLETE'
    statusCode: 502
    sanitized: 'full'
    reason: 'Failed to complete the S3 multipart upload.'
  }
  {
    errorCode: 'ERR_BLOB_DOWNLOAD'
    statusCode: 500
    sanitized: 'full'
    reason: 'Problem with blob download.'
  }

  {
    errorCode: 'ERR_UNKNOWN_MASTER_KEY'
    statusCode: 500
    reason: (ctx) -> "Unknown master key id '#{ctx.masterkeyid}'."
    contextPattern:
      masterkeyid: String
  }
  {
    errorCode: 'ERR_UNKNOWN_USERID'
    statusCode: 404
    reason: (ctx) -> "Could not find user id '#{ctx.uid}'."
    contextPattern:
      uid: String
  }
  {
    errorCode: 'ERR_UNKNOWN_USERNAME'
    statusCode: 404
    reason: (ctx) -> "Could not find user '#{ctx.username}'."
    contextPattern:
      username: String
  }
  {
    errorCode: 'ERR_UNKNOWN_KEYID'
    statusCode: 404
    reason: (ctx) -> "Could not find key id '#{ctx.keyid}'."
    contextPattern:
      keyid: String
  }

  {
    errorCode: 'ERR_AUTH_FIELD_MISSING'
    statusCode: 401
    sanitized: 'full'
    reason: (ctx) -> "Invalid signature (missing #{ctx.missing})."
    contextPattern:
      missing: String
  }
  {
    errorCode: 'ERR_AUTH_DATE_INVALID'
    statusCode: 401
    sanitized: 'full'
    reason: (ctx) -> "Invalid authdate"
  }
  {
    errorCode: 'ERR_AUTH_SIG_EXPIRED'
    statusCode: 401
    sanitized: 'full'
    reason: (ctx) -> 'Expired signature'
  }
  {
    errorCode: 'ERR_AUTH_KEY_UNKNOWN'
    statusCode: 401
    sanitized: 'full'
    reason: (ctx) -> "Unknown key."
  }
  {
    errorCode: 'ERR_AUTH_SIG_INVALID'
    statusCode: 401
    sanitized: 'full'
    reason: (ctx) -> "Invalid signature."
  }
  {
    errorCode: 'ERR_AUTH_EXPIRES_INVALID'
    statusCode: 401
    sanitized: 'full'
    reason: (ctx) -> "Invalid expires."
  }
  {
    errorCode: 'ERR_AUTH_NONCE_INVALID'
    statusCode: 401
    sanitized: 'full'
    reason: (ctx) -> "Invalid nonce."
  }

  {
    errorCode: 'ERR_ACCESS_DENY'
    statusCode: 404
    sanitized: 'full'
    reason: 'Access denied by policy.'
  }
  {
    errorCode: 'ERR_ACCESS_DEFAULT_DENY'
    statusCode: 404
    sanitized: 'full'
    reason: 'Access denied without policy.'
  }

  {
    errorCode: 'ERR_APIKEY_CREATE'
    statusCode: 403
    sanitized: 'full'
    reason: 'Failed to create API key.'
  }
  {
    errorCode: 'ERR_APIKEY_DELETE'
    statusCode: 403
    sanitized: 'full'
    reason: 'Failed to create API key.'
  }

  {
    errorCode: 'ERR_CONTENT_REPO_EXISTS'
    statusCode: 409
    sanitized: 'full'
    reason: (ctx) -> "The repo `#{ctx.repoFullName}` already exists."
    contextPattern:
      repoFullName: String
  }
  {
    errorCode: 'ERR_CONTENT_MISSING'
    statusCode: 404,
    sanitized: 'full'
    reason: (ctx) ->
      if ctx.object?
        "The object `#{ctx.object}` is missing."
      else if ctx.tree?
        "The tree `#{ctx.tree}` is missing."
      else if ctx.blob?
        "The blob `#{ctx.blob}` is missing."
      else if ctx.commit?
        "The commit `#{ctx.commit}` is missing."
      else
        "Some content is missing."
  }
  {
    errorCode: 'ERR_CONTENT_CHECKSUM'
    statusCode: 500,
    sanitized: 'full'
    reason: (ctx) ->
      "Content id checksum error for #{ctx.type} #{ctx.sha1}."
    contextPattern:
      sha1: String
      type: String
  }
  {
    errorCode: 'ERR_REPO_MISSING'
    statusCode: 404,
    sanitized: 'full'
    reason: 'The repo does not exist.'
  }
  {
    errorCode: 'ERR_REF_MISMATCH'
    statusCode: 409
    sanitized: 'full'
    reason: 'The old ref does not match.'
  }
  {
    errorCode: 'ERR_REF_NOT_FOUND'
    statusCode: 404
    sanitized: 'full'
    reason: (ctx) -> "The requested ref '#{ctx.refName}' could not be found."
    contextPattern:
      refName: String
  }

  {
    errorCode: 'ERR_CONFLICT'
    statusCode: 409
    sanitized: 'full'
    reason: 'The request conflicts with a concurrent request.'
  }

  {
    errorCode: 'ERR_LOST_LOCK'
    statusCode: 409
    sanitized: 'full'
    reason: 'The active request lost its lock to a concurrent request.'
  }

  {
    errorCode: 'ERR_LOGIC'
    statusCode: 500
    sanitized: null
    reason: 'There is a problem with the program logic.'
  }

  {
    errorCode: 'ERR_CREATE_ACCOUNT_USERNAME'
    statusCode: 401
    sanitized: 'full'
    reason: 'Cannot create account: no username.'
  }
  {
    errorCode: 'ERR_CREATE_ACCOUNT_USERNAME_TOOSHORT'
    statusCode: 401
    sanitized: 'full'
    reason: 'Username must be at least 3 characters long.'
  }
  {
    errorCode: 'ERR_CREATE_ACCOUNT_USERNAME_INVALID'
    statusCode: 401
    sanitized: 'full'
    reason: 'Username may only contain the following characters: "a-z", "0-9",  "_", and "-".'
  }
  {
    errorCode: 'ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED'
    statusCode: 401
    sanitized: 'full'
    reason: 'Username is not allowed.'
  }
  {
    errorCode: 'ERR_CREATE_ACCOUNT_USERNAME_GITHUB'
    statusCode: 401
    sanitized: 'full'
    reason: 'Username already exists as a non-github account.'
  }
  {
    errorCode: 'ERR_CREATE_ACCOUNT_USERNAME_GITIMP'
    statusCode: 401
    sanitized: 'full'
    reason: 'Username already exists as a non-gitimp account.'
  }
  {
    errorCode: 'ERR_CREATE_ACCOUNT_USERNAME_GITZIB'
    statusCode: 401
    sanitized: 'full'
    reason: 'Username already exists as a non-gitzib account.'
  }
  {
    errorCode: 'ERR_CREATE_ACCOUNT_EMAIL'
    statusCode: 401
    sanitized: 'full'
    reason: 'Cannot create account: no email address.'
  }
  {
    errorCode: 'ERR_ACCOUNT_DELETE'
    statusCode: 403
    sanitized: 'full'
    reason: 'Cannot delete account.'
  }

  {
    errorCode: 'ERR_CREATE'
    statusCode: 400
    sanitized: 'full'
    reason: 'Failed to create a resource.'
  }
  {
    errorCode: 'ERR_UNKNOWN'
    statusCode: 404
    sanitized: 'full'
    reason: 'A resource is unknown.'
  }
  {
    errorCode: 'ERR_UPDATE'
    statusCode: 400
    sanitized: 'full'
    reason: 'An update failed.'
  }

  # Example: `meta.workspace` should be an object, but it's not.
  {
    errorCode: 'ERR_NOT_OF_KIND'
    statusCode: 400
    sanitized: 'full'
    reason: 'An entry is not of the expected kind.'
  }

  {
    errorCode: 'ERR_PROC_TIMEOUT'
    statusCode: 503
    sanitized: 'full'
    reason: 'Request processing took too long.'
  }

  {
    errorCode: 'ERR_API_VERSION'
    statusCode: 409
    sanitized: 'full'
    reason: 'The API version is incompatible.'
  }

  {
    errorCode: 'ERR_UPDATE_SYNCHRO'
    statusCode: 500
    sanitized: null
    reason: 'An update on a synchro failed.'
  }

  {
    errorCode: 'ERR_SYNCHRO_MISSING'
    statusCode: 404,
    sanitized: null
    reason: 'The synchro does not exist.'
  }
  {
    errorCode: 'ERR_SYNCHRO_CONTENT_MISSING'
    statusCode: 404,
    sanitized: null
    reason: (ctx) ->
      if ctx.object?
        "The synchro object `#{ctx.object}` is missing."
      else if ctx.tree?
        "The synchro tree `#{ctx.tree}` is missing."
      else if ctx.commit?
        "The synchro commit `#{ctx.commit}` is missing."
      else
        "Some synchro content is missing."
  }
  {
    errorCode: 'ERR_SYNCHRO_STATE'
    statusCode: 400,
    sanitized: null
    reason: 'The synchro state is invalid.'
  }
  {
    errorCode: 'ERR_SYNCHRO_SNAPSHOT_INVALID'
    statusCode: 500,
    sanitized: null
    reason: 'The synchro snapshot is invalid.'
  }
  {
    errorCode: 'ERR_SYNCHRO_APPLY_FAILED'
    statusCode: 500,
    sanitized: null
    reason: 'Sync apply failed.'
  }

]

for s in specs
  NogError[s.errorCode] = s
