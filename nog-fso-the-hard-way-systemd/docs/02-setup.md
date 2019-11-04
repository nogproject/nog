# Setup
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Unless you are already in a separate directory, create one:

```bash
mkdir nog-fso-the-hard-way-systemd
cd nog-fso-the-hard-way-systemd
```

The tutorials use Kubernetes to run several services and a Vagrant VM to
simulate a storage host that would be run as physical or virtual machine in
a production setup.

The tutorials assume that you use a local testing Kubernetes, such as Docker on
Mac or Minikube.  `docker` and `kubectl` must be configured such that they both
connect to the same Docker daemon.
