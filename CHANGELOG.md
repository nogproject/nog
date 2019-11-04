# Nog -- CHANGELOG
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Introduction

This log describes the change history of repo `nog`, which contains:

 - the Nog web app <https://nog.zib.de>
 - related backend daemons
 - related libraries

The log sections describe releases of the repo.  Within each section, notes are
grouped by topic.

Some parts may use versions that change independently.  See `versions.yml`.

## nog-0.4.0, 2019-11-01

GIT RANGE: `05f2f3ddad..cfa3cdde64`

Program versions:

* See source for Meteor packages and nogappd
* `nogreplicad-0.3.1`, unchanged,
  see [nogreplicad/CHANGELOG](./daemons/nogreplicad/CHANGELOG.md)
* `nogsumd-0.3.1`, unchanged,
  see [nogsumd/CHANGELOG](./daemons/nogsumd/CHANGELOG.md)
* `nog-app-2-0.1.0`, unchanged
* `git-fso-0.1.0`, unchanged
* `nogfsoctl-0.3.0`, unchanged
* `nogfsog2nd-0.1.0`, unchanged
* `nogfsoregd-0.3.0`, unchanged
* `nogfsoschd-0.3.0`, unchanged
* `nogfsosdwbakd3-0.2.0`, unchanged
* `nogfsosdwgctd-0.1.0`, unchanged
* `nogfsostad-0.4.0`, updated
* `nogfsostaudod-0.2.0`, unchanged
* `nogfsostasvsd-0.1.0`, unchanged
* `nogfsotard-0.2.0`, unchanged
* `nogfsotargctd-0.2.0`, unchanged
* `nogfsotarsecbakd-0.2.0`, unchanged
* `nogfsotchd3-0.1.0`, unchanged
* `nogfsodomd-0.1.0`, unchanged
* `tartt-0.3.0`, unchanged

nogfso-0.4.0, 2019-11-01:

* New mandatory option `nogfsostad --stdtools-projects-root` to avoid
  hard-coded Stdtools project root path

## nog-0.3.0, 2019-10-28

GIT RANGE: `2aeb045f32..384fa3a938`

Program versions:

* See source for Meteor packages and nogappd
* `nogreplicad-0.3.1`, unchanged,
  see [nogreplicad/CHANGELOG](./daemons/nogreplicad/CHANGELOG.md)
* `nogsumd-0.3.1`, unchanged,
  see [nogsumd/CHANGELOG](./daemons/nogsumd/CHANGELOG.md)
* `nog-app-2-0.1.0`, new
* `git-fso-0.1.0`, unchanged
* `nogfsoctl-0.3.0`, updated
* `nogfsog2nd-0.1.0`, unchanged
* `nogfsoregd-0.3.0`, updated
* `nogfsoschd-0.3.0`, updated
* `nogfsosdwbakd3-0.2.0`, updated
* `nogfsosdwgctd-0.1.0`, unchanged
* `nogfsostad-0.3.0`, updated
* `nogfsostaudod-0.2.0`, updated
* `nogfsostasvsd-0.1.0`, new
* `nogfsotard-0.2.0`, updated
* `nogfsotargctd-0.2.0`, updated
* `nogfsotarsecbakd-0.2.0`, updated
* `nogfsotchd3-0.1.0`, unchanged
* `nogfsodomd-0.1.0`, new
* `tartt-0.3.0`, updated

General changes:

* Go 1.12.6
* Nog FSO the hard way tutorials
* GitLab CI
* The Git history has been cut in preparation for publishing to a wider
  audience.  The full history before nog-0.3.0 is preserved in the internal ZIB
  repo.
* Most Markdown titles have been changed to H1 with H2 sections.

nog-app-2-0.1.0, 2019-10-28:

* New Meteor application to manage access tokens

nogfso-0.3.0, 2019-10-28:

* New supervisor tool `nogfsostasvsd` to run daemons with a list of numeric
  supplementary groups that is determined from a group naming convention.  The
  daemon is restarted if the list of groups changes.
* `nogfsoregd` supports MongoDB TLS.
* `nogfsotarsecbakd` uses compliant ISO 8601 basic format.
* `nogfsosdwbakd3` uses compliant ISO 8601 basic format.
* FSO registry supports Unix domain information, which is synced by
  `nogfsodomd`.
* archive-repo and unarchive-repo workflows
* freeze-repo and unfreeze repo workflows
* FSO registry event journal trimming and garbage collection
* New split-root workflow to create new repos based on disk usage
* Archive encryption can be controlled per root, from where the configuration
  propagates to newly initialized repos.
* `nogfsoctl get repo` supports per-repo GPG keys.
* `nogfsotarsecbakd` no longer encrypts tars, assuming that the original files
  are encrypted, like `secret.asc`, if encryption is desired.
* `nogfsoschd`, `nogfsotard`, `nogfsosdwbakd3` support per-repo GPG keys.
* Enabled 2019 stdrepos for `nogfsoctl find-untracked`
* New ping-registry workflow, which illustrates how to coordinate actions
  between multiple servers and an admin
* Structured errors in Packages `events` and `pingregistrywf`
* `nogfsostad` starts `nogfsostaudod-fd` via `nogfsostasududod`

tartt-0.3.0, 2019-10-28:

* `tartt` has a new option `--plaintext-secret` to encrypt tars and save
  a plaintext secret.  This combination can be useful when combining insecure
  tar storage with secure tartt repo storage.
* `tartt` now uses AES128 by default; AES192 and AES256 can be configured with
  the new option `--cipher-algo`.

## nog-0.2.0, 2018-12-18

GIT RANGE: `500d26be3c..cb04c72651`

Program versions:

* See source for Meteor packages and nogappd
* `nogreplicad-0.3.1`, updated,
  see [nogreplicad/CHANGELOG](./daemons/nogreplicad/CHANGELOG.md)
* `nogsumd-0.3.1`, updated,
  see [nogsumd/CHANGELOG](./daemons/nogsumd/CHANGELOG.md)
* `git-fso-0.1.0`, implementation unchanged, new in list of program versions
* `nogfsoctl-0.2.0`, updated
* `nogfsog2nd-0.1.0`, unchanged
* `nogfsoregd-0.2.0`, updated
* `nogfsoschd-0.2.0`, updated
* `nogfsosdwbakd3-0.1.0`, new
* `nogfsosdwgctd-0.1.0`, new
* `nogfsostad-0.2.0`, updated
* `nogfsostaudod-0.1.0`, new
* `nogfsotard-0.1.0`, new
* `nogfsotargctd-0.1.0`, new
* `nogfsotarsecbakd-0.1.0`, new
* `nogfsotchd3-0.1.0`, new
* `tartt-0.2.0`, updated

General changes:

* The repo now uses Go 1.11 with modules.
* The repo has been split into `nog` and `nog-sup` in preparation for
  publishing the Git history to a wider audience.

nogfso-0.2.0, 2018-12-18:

* Proof of concept how Nogfsostad acts as a user, aka "user do", aka sudo
  without s, aka "udo".
* JWTs now support a new `xcrd` claim to pass username and groups by LDAP
  domain.
* New subcommand `nogfsoctl tartt config` to inspect `tarttconfig.yml`
* `git-fso` reports error to compute total tree size.
* New subdir tracking option `git-fso --ignore-most` to track only toplevel
* New subcommand `nogfsoctl move-shadow-backup` and event
  `EV_FSO_SHADOW_BACKUP_REPO_MOVED` to change the shadow repo backup location
* New move-repo workflow to simultaneously change the file data and shadow repo
  locations
* New move-shadow workflow to change shadow repo location
* Several variants how to observe the registry and repo event history have been
  implemented in `nogfsostad`; see Git history `gitk --
  'backend/internal/nogfsostad/observer*'` for details.
* The `Stdtools2017` naming rule handles project toplevel as repo.
* The `nogfsoregd` event journal now ensures that either all events of
  a command are observed or none.
* `nogfsostad` now runs `git-fso stat --mtime-range-only` as a regular
  background task.
* `nogfsostad` now runs `git gc` as a regular background task.
* Several new daemons for archival, backup, and garbage collection:
  `nogfsosdwbakd3`, `nogfsosdwgctd`, `nogfsotard`, `nogfsotargctd`,
  `nogfsotarsecbakd`, `nogfsotchd3`
* New event `EV_FSO_TARTT_REPO_CREATED`, gRPC `repos.InitTartt()`, `nogfsoctl
  init-tartt`
* New event `EV_FSO_SHADOW_BACKUP_REPO_CREATED`, gRPC
  `repos.InitShadowBackup()`, `nogfsoctl init-shadow-backup`

tartt-0.2.0, 2018-12-18:

* `tartt` now uses `tar --listed-incremental-mtime` if available to ignore
  ctime changes.  The new tar option is available in the patched GNU tar from
  <https://github.com/sprohaska/gnu-tar> branch `next^`.  It works like
  `--listed-incremental` but uses only mtime to detect modified files, ignoring
  ctime.
* `tartt` now supports disabled levels, which can be used to migrate the level
  config to new level names: add new levels, disable the old levels, wait until
  the old levels have been fully garbage-collected, remove the old levels from
  the config.
* `tartt gc` now removes incomplete archives that are older than 5 days.
* `tartt` now uses locale `C.UTF-8`, which is useful with container base
  images, which may not have `en_US.UTF-8`, for example Debian and Ubuntu.
* `tartt tar` now handles a missing origin gracefully: it saves an empty
  placeholder archive instead of failing.
* `tartt init` level 0 default has been changed to interval 2mo / lifetime 6mo
  in order to reduce mid-term data amplification
* `tartt` has a new option `--notify-preload-secrets-done` to notify a FIFO
  after secret preloading has completed.
* `tartt` decrypts and decompresses in parallel.
* `tartt` now enforces restoring owner and permissions unless told otherwise
  with `--no-same-owner` or `--no-same-permissions`.
* `tartt restore ... <members>...` can be used to restore only selected archive
  members.
* `tartt` has a new subcommand `ls-tar` to list tar members.
* `tartt` now handles tar "file changed as we read it" as a non-fatal error.
* `tartt` now uses power-of-two sizes, specifically a max chunk size of 128 MiB
  and a max piece size of 256 GiB.
* `tartt` more robustly handles child process errors.
* `tartt` now uses `tar --sparse` to efficiently store sparse files.
* `tartt` now uses `tar --no-check-device` to avoid spurious big tarballs.
* `tartt` supports a new storage driver `localtape`, which writes tar archives
  to a separate directory, outside of the Tartt repo.

## nog-0.1.0, 2018-06-11

GIT RANGE: until commit `500d26be3c`

Program versions:

* See source for Meteor packages and app.
* nogfsoctl-0.1.0, new
* nogfsog2nd-0.1.0, new
* nogfsoregd-0.1.0, new
* nogfsoschd-0.1.0, new
* nogfsostad-0.1.0, new
* tartt-0.1.0, new

nogfso-0.1.0, 2018-06-11:

* Towards production
* First version that has been deployed at ZIB as a preview

tartt-0.1.0, 2018-06-11:

* Backup and archival proof of concept, see NOE-20

nogfso-0.0.32, 2018-01-12:

* Proof of concept, see NOE-13

## Truncated

See internal ZIB repo for historic log.
