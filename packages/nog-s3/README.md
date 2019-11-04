# Package `nog-s3` (DEPRECATED)

DEPRECATED: `nog-s3` should not be used anymore.  Use `nog-multi-bucket`
instead.

`nog-s3` wraps just enough of the AWS SDK to implement `nog-blob`.  `S3` exposes
only a few sync functions.  Errors are translated to `NogError.Error`.  The
`opts` are identical to params of the official AWS SDK:
<http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/S3.html>.

## `S3.configure(opts)` (server)

`configure()` updates the active configuration with the provided `opts`:

 - `accessKeyId` (`String`, default `Meteor.settings.AWSAccessKeyId`).
 - `secretAccessKey` (`String`, default `Meteor.settings.AWSSecretAccessKey`).
 - `region` (`String`, default `Meteor.settings.AWSBucketRegion`).
 - `signatureVersion` (`s3` or `v4`, default
   `Meteor.settings.AWSSignatureVersion` or `v4`): `eu-central-1` requires
   `v4`.
 - `s3ForcePathStyle` (`Boolean`, default
   `Meteor.settings.AWSS3ForcePathStyle` or `false`): URL format for `false` is
   `{bucket}.{region}...`; URL format for `true` is `{endpoint}/{bucket}`.  The
   path style may be useful with alternative S3 implementations, like Ceph
   RadosGW.
 - `endpoint` (`String`, default `Meteor.settings.AWSEndpoint`): The endpoint
   must accept requests from the server and from client browsers.
 - `sslEnabled` (`Boolean`, default `Meteor.settings.AWSSslEnabled` or `true`).
 - `ca` (`String`, default `Meteor.settings.AWSCa`): If present, must be an
   absolute path to a CA certificate bundle .pem file, which will be loaded and
   used instead of the CAs that are bundled with Node.

The key needs to have `s3:PutObject` and `s3:GetObject` rights on the S3 bucket
that is used by `nog-blob`.  The recommended approach to AWS permission
management is to use one AWS IAM user for the application and grant rights via
groups with inline policies (use the custom policy editor).  For example:

User `nog-app`.

Group `nog-s3-get` with policy:

    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Sid": "Stmt1420905603000",
                "Effect": "Allow",
                "Action": [
                    "s3:GetObject"
                ],
                "Resource": [
                    "arn:aws:s3:::nog/*"
                ]
            }
        ]
    }

Group `nog-s3-put` with policy:

    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Sid": "Stmt1420905603000",
                "Effect": "Allow",
                "Action": [
                    "s3:PutObject"
                ],
                "Resource": [
                    "arn:aws:s3:::nog/*"
                ]
            }
        ]
    }

The S3 CORS configuration must allow any origin and expose the ETag header (see
<http://docs.aws.amazon.com/AWSJavaScriptSDK/guide/browser-configuring.html#Cross-Origin_Resource_Sharing__CORS_>):

    <?xml version="1.0" encoding="UTF-8"?>
    <CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
        <CORSRule>
            <AllowedOrigin>*</AllowedOrigin>
            <AllowedMethod>PUT</AllowedMethod>
            <AllowedMethod>POST</AllowedMethod>
            <AllowedMethod>GET</AllowedMethod>
            <AllowedMethod>HEAD</AllowedMethod>
            <MaxAgeSeconds>3000</MaxAgeSeconds>
            <AllowedHeader>*</AllowedHeader>
            <ExposeHeader>ETag</ExposeHeader>
        </CORSRule>
    </CORSConfiguration>

## AWS SDK

### `S3.createMultipartUpload(opts)` (server)

See
<http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/S3.html#createMultipartUpload-property>.

### `S3.getSignedUploadPartUrl(opts)` (server)

See
<http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/S3.html#uploadPart-property>.

### `S3.getSignedDownloadUrl(opts)` (server)

See
<http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/S3.html#getObject-property>.

### `S3.completeMultipartUpload(opts)` (server)

See
<http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/S3.html#completeMultipartUpload-property>.

### `S3.abortMultipartUpload(opts)` (server)

See
<http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/S3.html#abortMultipartUpload-property>.

