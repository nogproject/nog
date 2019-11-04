# Overview and Background Information
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Nog FSO System Overview

Nog File System Observer (short Nog FSO or FSO) is a system to track and manage
research data on filesystems.  It tracks directories of related files, not
individual files.  On the highest level, Nog FSO consists of three related
services that each may be implemented as several service daemons:

* Web application: currently, identity provider and access control; in the
  future, also user interface to control operations such as data archival.
* FSO registry: metadata store; gateway from web application to the storage
  server.
* FSO storage server: online and tape filesystem; several service daemons
  observe and manage the filesystems.

Nog FSO is described in NOE design documents.  The locations to access NOEs are
listed in a separate section below.

NOE-13 and NOE-20 together provide an overview of the system:

* NOE-13 -- Git Filesystem Observer
* NOE-20 -- Git Filesystem Observer Backup and Archival

Further NOEs that might be helpful to get a more detailed overview:

* NOE-22 -- Filesystem Observer User Privilege Separation
* NOE-23 -- Git Filesystem Observer Repo Freeze
* NOE-24 -- Git Filesystem Observer Repo Archive
* NOE-25 -- Tracking Unix Users and Groups in the Filesystem Observer Registry

## Repositories

Full repositories with all branches are available on the internal filesystem of
the Visual Data Analysis ZIB department.  Selected branches of some
repositories are also pushed to Git hosting services.

NOE design documents:

* <https://git.imp.fu-berlin.de/bcp/noe.git>

Main source code repository:

* <https://git.zib.de/nog/nog.git>

BCPFS command line tools source code:

* <https://git.imp.fu-berlin.de/bcp/bcpfs.git>
