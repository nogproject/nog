# Developing Nog Backend Services in Go

## Introduction

Make is primarily used for Go.  Npm scripts are used for JavaScript.  The two
approaches should be incrementally unified when there are opportunities.

Only the sub-directory `backend/` contains Go code.  It uses a typical Go
layout with `cmd/`, `internal/`, `pkg/`, and `vendor/`.  See section on Go
style for more.

We may add additional Go sub-directories if needed.

## General Docker godev workflow

The project directory is bind-mounted into the container.  It should just work
if you run Docker on directly on a Linux host or use Docker Mac.  See separate
section below if you use Docker Machine with VirtualBox.

Aliases:

```bash
source ./tools/env.sh
```

`.git` must be the git dir in order for Git commands to work in the godev
container.  See `tools/env.sh` for a sequence of commands to replace a git file
with the corresponding git dir.

To start or restart from scratch:

```bash
make clean images
make
```

Makefile usage:

```bash
make help
```

Init vendor and build shared binaries:

```bash
make
```

Build static binaries:

```bash
make binaries
```

Clean up old containers:

```bash
make gc
```

Clean up Docker objects that can be quickly re-created:

```bash
make down
```

Full cleanup, including stateful volumes:

```bash
make down-state
```

State is maintained in volumes.

**You must delete the `go` volume after modifying the `godev` image, so that
the volume is re-created with the updated image content:**

```
make gc
ddev ls -l /go/bin
docker volume rm fuimagesnog2016_go
ddev ls -l /go/bin
```

Godoc:

```bash
make up-godoc
open http://localhost:6060/pkg/github.com/nogproject/?m=all
```

### Using Docker in VirtualBox

If you run Docker in VirtualBox, you need the additional configuration
described in this section.

Docker containers must be able to bind-mount the Git working tree.  To allow
that, the filesystem must be first shared to VirtualBox, so that the Linux
kernel in the VM can then bind-mount it into containers.  Assuming your Git
working tree is on `/local`, create the virtual machine as follows:

```bash
docker-machine create --driver virtualbox --virtualbox-share-folder '/local:/local' default
```

Docker exposes container ports to the VM.  In order to access them from your
main host, you need to forward the ports.  This is necessary so that the Meteor
app, for example, can access services, like Minio or Nog FSO.  Use SSH port
forwarding as follows, usually for all the ports listed `docker-compose.yml`
(search for `ports`):

```bash
docker-machine ssh default \
    -L 6060:localhost:6060 \
    -L 7540:localhost:7540 \
    -L 7550:localhost:7550 \
    -L 7552:localhost:7552 \
    -L 7554:localhost:7554 \
    -L 10080:localhost:10080 \
    -L 10180:localhost:10180 \
    ;
```

Some containers need to reach the Meteor app running on the host.  Set
`DOCKER_TO_HOST_ADDR` to an appropriate value.  See `docker-compose.yml`.  Add
details here if you successfully used `DOCKER_TO_HOST_ADDR`.

## Go style

Keep this section short.

Follow [Effective Go](https://golang.org/doc/effective_go.html) and the [Go
Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

### Vendor

We rely on Dep and do not track `vendor/` in Git.  A build, thus, may require
network access.  Switching between commits may require an explicit `make
vendor`.  But our Git history contains no vendor noise.

Relying on Dep seems to be a good trade-off.  If we want to track `vendor/`
more rigorously in the future, we would consider tracking it in a Git submodule
to avoid the vendor noise in the main repo.

### Protobuf

We rely on Protoc to compile `.proto` files to `.pb.go` files and do not track
`.pb.go` files in Git.  It avoids noise in the Git history; a single `make`
just works; and there are no dependent packages that would rely on compiled
`.pb.go` files.

### Go package layout

Focus on decoupling when considering a package layout.  `cmd/`, `internal/`,
and `pkg/` are possible locations.  The Bill Kennedy way seems overall more
useful than the Ben Johnson way.  See references.

In particular, do not introduce a central package with domain types.  Accept
interfaces and return structs.  Some type duplication is acceptable if it
improves decoupling.  Err on the side of over packaging, since packages are
often difficult to split later.  But be pragmatic: it can be reasonable to
import sibling packages.

Packages in `pkg/` should be more on the kit side.  They should be generic and
candidates for importing from the outside.  But the outside should not import
a package unless the package states that it is ready to be imported.

Packages in `internal/` are more on the application side.  For example, server
implementation could be placed in `internal/`, since it should not be used from
the outside; while protos and client packages could be placed in `pkg/`,
because they are more likely to be used from the outside.

Packages that are imported from the outside, such as Zap or GRPC, should be
wrapped, either on the general `pkg/` or `internal/` level or below topic
packages, depending on how specific they are.  Examples in repo `bcpfs`:
`pkg/zap`, `internal/suc/grpcd`.

Packages that are closely related to outside packages should be place in the
`pkg/` tree under a topic.  Example in repo `bcpfs`: `pkg/grpc/ucred/`.

Protobuf Buffer Go packages should be indicated by `pb`.  But the Protocol
Buffer package itself should not use `pb`.  Example:
`pkg/nogechopb/nogecho.proto`:

```
syntax = "proto3";
package nogecho;
option go_package = "nogechopb";
```

Packages that are related to servers should be indicated by `d`.  Example:
`internal/nogechod/`.

References:

* Carlisia Pinto, Go and a Package Focused Design,
  <https://blog.gopheracademy.com/advent-2016/go-and-package-focused-design/>.
* Bill Kennedy, Package Oriented Design,
  <https://www.goinggo.net/2017/02/package-oriented-design.html>.
* Ben Johnson, Standard Package Layout,
  <https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1>.

## Performance issues with Docker Machine + VirtualBox on Linux

We observed bad performance and spurious connection errors when using the
default settings of Docker Machine on Linux.  Create the VM with at least 8 GB
RAM and 2 CPUs:

```
docker-machine create \
  --driver virtualbox \
  --virtualbox-memory 8192 \
  --virtualbox-cpu-count 2 \
  <MACHINE-NAME>
```

If you observe slow `docker pull` downloads, try 1 CPU instead.

See also: <https://docs.docker.com/machine/drivers/virtualbox/#options>.

## Bind-mounting directories with Docker Machine + VirtualBox on Linux

On Linux systems that use Docker Machine to run Docker containers,
bind-mounting volumes does not work off-the-shelf.  You need to create a Shared
Folder between host and virtual machine where the name of the share is equal to
the name of the top-level directory under which your bind-mounted directories
reside.  The directory is then automatically mounted inside the VM under
`/<SHARE-NAME>`.  This results in Docker seeing the same directory structure on
the host and inside the VM.

Assuming that the directories to be bind-mounted are below `/cache/201x/...`,
create a VM with:

```
docker-machine create --driver virtualbox <MACHINE-NAME>
```

and mount your local `/cache` folder on the VM:

```
VBoxManage sharedfolder add <MACHINE-NAME> --name local --hostpath /local --automount
```

Start the VM:

```
docker-machine start <MACHINE-NAME>
```

The folder is now mounted inside the VM under `/cache`.  You can verify it by
logging into the machine with `docker-machine ssh <MACHINE-NAME>`.  If the
directory is there, but has no content, ensure that the directory is owned by
user docker in group staff.  For example, the group/owner was `root/root`.
I stopped the VM, removed the shared folder, created it again, started the VM,
then it worked.

You can now run Docker containers as usual, bind-mounting should work.

Tested using docker-machine version 0.9.0.

## Port forwarding with Docker Machine

When running Docker Machine with VirtualBox, exposed container ports are only
available in the VM but not directly on localhost.  To forward ports via SSH
run:

```
docker-machine ssh <MACHINE-NAME> -L <PORT>:localhost:<PORT> ...
```

Alternatively, you can also use the standard SSH options `-D` and `-R`.

## nogfso

See [backend/HACKING-fso](./fso/HACKING-fso.md)
