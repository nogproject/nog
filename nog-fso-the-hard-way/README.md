# Nog FSO the Hard Way
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Introduction

This tutorial teaches how to operate Nog File System Observer the hard way.  It
explains on a basic level how to bootstrap an installation in order to
facilitate learning.  This is not how you would deploy the system in
production.

## Tutorials

The tutorials use Docker containers to illustrate a multi-host setup.  It
should be straightforward to adjust the tutorial for a multi-VM or a single
host environment.

1. [Overview and Background Information](./docs/01-intro.md)
1. [Setup](./docs/02-setup.md)
1. [Getting Software](./docs/03-software.md)
1. [Generating X.509 Certificates](./docs/04-certs.md)
1. [Bootstrapping the Web Application](./docs/05-web-app.md)
1. [Bootstrapping the FSO Registry Daemon](./docs/06-nogfsoregd.md)
1. [Bootstrapping the Admin Command Line Tool](./docs/07-nogfsoctl.md)
1. [Initializing the FSO Registries](./docs/08-registries.md)
1. [Bootstrapping the File Server Directory Layout](./docs/09-storage-layout.md)
1. [Bootstrapping the Main File Server Daemon](./docs/10-nogfsostad.md)
1. [Initializing FSO Roots and Repos](./docs/11-roots-and-repos.md)
1. [Bootstrapping the Tar Daemon](./docs/12-nogfsotard.md)
1. [Bootstrapping the Shadow Backup Daemon](./docs/13-nogfsosdwbakd3.md)
1. [Bootstrapping the Tar Secrets Backup Daemon](./docs/14-nogfsotarsecbakd.md)
1. [Bootstrapping the Sudo Helper Daemon](./docs/15-nogfsostaudod.md)
1. [Freezing and Unfreezing Repos](./docs/16-freeze.md)
1. [Bootstrapping the Restore Daemon](./docs/17-nogfsorstd.md)
1. [Archiving and Unarchiving Repos](./docs/18-archive.md)
1. [Bootstrapping the Tar GC Daemon](./docs/19-nogfsotargctd.md)
1. [Bootstrapping the Unix Domain Daemon](./docs/20-nogfsodomd.md)
1. [Using Nog FSO](./docs/21-using-fso.md)
1. [Cleanup](./docs/99-cleanup.md)
