# Nog Daemons

Nog daemons are long-running processes that support the main Nog application.
They are implemented in Python for execution in Docker containers.

 - `nogreplicad` monitors the blob collection in a Nog MongoDB and replicates
   blobs to the configured object buckets.
 - `nogsumd` verifies S3 ETags for blob locations and verifies that the SHA1
   matches the blob id.  It also computes a SHA256 and stores it on the blob.

See Python doc strings in the sub-directories for details.
