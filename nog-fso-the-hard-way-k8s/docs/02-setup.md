# Setup
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Unless you are already in a separate directory, create one:

```bash
mkdir nog-fso-the-hard-way-k8s
cd nog-fso-the-hard-way-k8s
```

The tutorials use Kubernetes to run several services and simulate hosts that
might be run as physical or virtual machines in a production setup.

The tutorials assume that you use a local testing Kubernetes, such as Docker on
Mac or Minikube.  `docker` and `kubectl` must be configured such that they both
connect to the same Docker daemon.
