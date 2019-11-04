# `cfgBackupDir` is the toplevel directory below which `nogfsotarsecbakd`
# creates sub-directories for the origins specified in `cfgOrigins`.
cfgBackupDir='/nogfso/backup/tartt'

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before a backup.  Backups are skipped if `df`
# reports less.
cfgCheckMinDf='
/nogfso/backup 1000000
'

# `cfgInterval` is the sleep time between backups.
cfgInterval='10m'

# `cfgOrigins` is a list of `<name> <dir> <find-args>...` lines.  `<name>` is
# the subdirectory below `cfgBackupDir` to back up files below `<dir>` that are
# selected by `find` with `<find-args>`.
cfgOrigins='
archive-tartt-secrets /nogfso/archive/tartt -name secret -o -name secret.asc
'

# `cfgBuckets` is a list of `<bucket> <max> <selector>...`.  The latest backup
# is added to `<bucket>` if `find -type f <selector>` does not match the most
# recent file in the bucket.  The oldest backups are deleted if a bucket
# contains more than `<max>` files.
#
# To ensure that the latest state is always in at least one bucket, bucket
# `latest` uses `-false`, so that it receives every backup.
cfgBuckets='
latest 2 -false
hourly 10 -mmin -60
daily 7 -mmin -1440
weekly 5 -mtime -7
monthly 4 -mtime -30
'
