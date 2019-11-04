# Package `nog-multi-bucket`

The package `nog-multi-bucket` provides mechanisms to manage blobs in multiple
object buckets.

`createBucketRouterFromSettings()` creates a multi-bucket router with methods
`getDownloadUrl({ blob, filename })` and `getImgSrc({ blob, filename })` that
return blob download URLs considering a configured bucket preference order and
bucket health.  The mechanism is used by package `nog-blob`.

Multi-bucket upload routing is supported through `createMultipartUpload({ key
})`, which uses the `writePrefs` settings (see below) to determine the upload
bucket.  `getSignedUploadPartUrl()` returns upload URLs for the individual
parts.  The upload is completed with `completeMultipartUpload()` or canceled
with `abortMultipartUpload()`.

## Meteor Settings

The multi-bucket router is configured from package `nog-blob` via
`Meteor.settings.multiBucket`.

`nog-multi-bucket` exports a Meteor check match pattern
`matchMultiBucketSettings`, which can be used to validate settings as follows:

```js
import { matchMultiBucketSettings } from 'meteor/nog-multi-bucket';
check(Meteor.settings.multiBucket, matchMultiBucketSettings);
```

The following are example settings for one AWS S3 bucket, which is assumed to
be always healthy, and one Ceph S3 bucket with a health check that reads an
object and verifies its content.  The Ceph S3 bucket is preferred for download
and upload:

```json
{
    "multiBucket": {
        "readPrefs": ["nog-zib-2", "nog"],
        "writePrefs": ["nog-zib-2", "nog"],
        "fallback": "nog",
        "buckets": [
            {
                "name": "nog",
                "region": "eu-central-1",
                "accessKeyId": "AKxxxxxxxxxxxxxxxxxx",
                "secretAccessKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
                "check": "healthy"
            },
            {
                "name": "nog-zib-2",
                "accessKeyId": "Cxxxxxxxxxxxxxxxxxxx",
                "secretAccessKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
                "endpoint": "https://objs2.zib.nogproject.io",
                "signatureVersion": "v4",
                "check": "getObject",
                "checkKey": "_whoami",
                "checkContent": "bucket:nog-zib-2",
                "checkInterval": "15s"
            }
        ]
    },
}
```

The health check expects an object whose content equals `checkContent`.  Such
an object can, for example, be created with a properly configured S3cmd as
follows:

```bash
echo 'bucket:nog-zib' >'_whoami'
s3cmd put '_whoami' 's3://nog-zib/_whoami'
```

## AWS S3 Configuration

The AWS key needs `s3:PutObject` and `s3:GetObject` rights on the S3 bucket
that is used by `nog-blob`.  The recommended approach to AWS permission
management is to use one AWS IAM user for the application and grant rights via
groups with inline policies (use the custom policy editor).  For example:

User `nog-app`.

Group `nog-s3-get` with policy:

```json
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
```

Group `nog-s3-put` with policy:

```json
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
```

The S3 CORS configuration must allow any origin and expose the ETag header (see
<http://docs.aws.amazon.com/AWSJavaScriptSDK/guide/browser-configuring.html#Cross-Origin_Resource_Sharing__CORS_>):

Use XML to configure CORS via the AWS Admin UI:

```xml
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
```

## Local Ceph S3 Developer Setup

A local Docker container with Ceph can be used as an alternative to AWS S3
during development.  The manual steps are described below.  The repo root
contains a Docker Compose file that automates the steps for a basic setup.

Run the Docker image `ceph/demo` as follows:

```bash
docker run -it --rm \
    --name ceph \
    -p 10080:80 \
    -e NETWORK_AUTO_DETECT=4 \
    -e CEPH_DEMO_UID=nog -e CEPH_DEMO_ACCESS_KEY=Cdemo -e CEPH_DEMO_SECRET_KEY=Cdemosecret -e CEPH_DEMO_BUCKET=noglocal \
    ceph/demo
```

Use the following entry in `Meteor.settings.multiBucket.buckets`:

```json
{
    "name": "noglocal",
    "endpoint": "http://localhost:10080",
    "accessKeyId": "Cdemo",
    "secretAccessKey": "Cdemosecret"
}
```

Configure an AWS credentials profile in `~/.aws/credentials`:

```ini
[cephdemo]
aws_access_key_id=Cdemo
aws_secret_access_key=Cdemosecret
```

Configure CORS settings.  With the following `cors.json`:

```json
{
    "CORSRules": [
        {
            "AllowedOrigins": ["*"],
            "AllowedMethods": ["PUT", "POST", "GET", "HEAD"],
            "MaxAgeSeconds": 3000,
            "AllowedHeaders": ["*"],
            "ExposeHeaders": ["ETag"]
        }
    ]
}
```

Run:

```bash
aws --profile cephdemo --endpoint-url http://localhost:10080 s3api put-bucket-cors --bucket noglocal --cors-configuration file://cors.json
```

You can create additional buckets as follows:

```bash
aws --profile cephdemo --endpoint-url http://localhost:10080 --region localhost s3 mb s3://noglocal2
```
