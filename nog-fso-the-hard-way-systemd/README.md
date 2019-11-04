# Nog FSO the Hard Way with Kubernetes and Systemd
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Introduction

This tutorial teaches how to operate Nog File System Observer the hard way
using Kubernetes for the web application and the FSO registry and a Vagrant VM
with Systemd for the storage server.  It explains on a basic level how to
bootstrap an installation in order to facilitate learning.  This is not how you
would deploy the system in production.

## Tutorials

The tutorials illustrate a setup that is close to a potential production setup.
The web application and the FSO registry run on Kubernetes.  The storage server
is simulated by a Vagrant VM with Systemd services.

1. [Overview and Background Information](./docs/01-intro.md)
1. [Setup](./docs/02-setup.md)
1. [Getting Software](./docs/03-software.md)
1. [Generating X.509 Certificates](./docs/04-certs.md)
1. [Bootstrapping MongoDB](./docs/05-mongodb.md)
1. [Bootstrapping the Web Application](./docs/06-web-app.md)
1. [Bootstrapping the FSO Registry Daemon](./docs/07-nogfsoregd.md)
1. [Bootstrapping the Admin Command Line Tool](./docs/08-nogfsoctl.md)
1. [Initializing the FSO Registries](./docs/09-registries.md)
1. [Bootstrapping the File Server VM](./docs/10-storage-vm.md)
1. [Bootstrapping the File Server Directory Layout](./docs/11-storage-layout.md)
1. [Bootstrapping the Main File Server Daemon](./docs/12-nogfsostad.md)
1. [Initializing FSO Roots and Repos](./docs/13-roots-and-repos.md)
1. [Bootstrapping the Tar Daemon](./docs/14-nogfsotard.md)
1. [Bootstrapping the Shadow Backup Daemon](./docs/15-nogfsosdwbakd3.md)
1. [Bootstrapping the Tar Secrets Backup Daemon](./docs/16-nogfsotarsecbakd.md)
1. [Bootstrapping the Sudo Helper Daemon](./docs/17-nogfsostaudod.md)
1. [Freezing and Unfreezing Repos](./docs/18-freeze.md)
1. [Bootstrapping the Restore Daemon](./docs/19-nogfsorstd.md)
1. [Archiving and Unarchiving Repos](./docs/20-archive.md)
1. [Bootstrapping the Tar GC Daemon](./docs/21-nogfsotargctd.md)
1. [Bootstrapping the Unix Domain Daemon](./docs/22-nogfsodomd.md)
1. [Using Nog FSO](./docs/23-using-fso.md)
1. [Cleanup](./docs/99-cleanup.md)
