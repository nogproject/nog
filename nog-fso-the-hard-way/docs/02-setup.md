# Setup
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Unless you are already in a separate directory, create one:

```bash
mkdir nog-fso-the-hard-way
cd nog-fso-the-hard-way
```

The tutorials use Docker containers to simulate several hosts, which are
connected via a network.  The tutorials usually use the more generic term host
to refer to the containers unless Docker-specific details are relevant.
Specific containers are referred to using their name, usually fully qualified,
for example host `nog.example.org`.

Create a Docker network and Docker volumes:

```bash
docker network create example.org
docker volume create example.org_online
docker volume create example.org_tape
```

Create Docker containers to simulate several hosts:

First terminal, host `nog.example.org`:

```bash
docker run -it \
    --name nog.example.org --hostname nog --network example.org \
    -v $PWD/local:/host/local \
    -p 8080:8080 \
    ubuntu:18.04
```

Second terminal, host `fso.example.org`:

```bash
docker run -it \
    --name fso.example.org --hostname fso --network example.org \
    -v $PWD/local:/host/local \
    ubuntu:18.04
```

Third terminal, host `storage.example.org`:

```bash
docker run -it \
    --name storage.example.org --hostname storage --network example.org \
    -v $PWD/local:/host/local \
    -v $PWD/tools:/host/tools \
    -v example.org_online:/srv/exorg_exsrv \
    -v example.org_tape:/srv/samfs \
    --cap-add CHOWN \
    --cap-add DAC_OVERRIDE \
    --cap-add DAC_READ_SEARCH \
    --cap-add FOWNER \
    --cap-add LINUX_IMMUTABLE \
    ubuntu:18.04
```

Fourth terminal, host `ops.example.org`:

```bash
docker run -it \
    --name ops.example.org --hostname ops --network example.org \
    -v $PWD/local:/host/local \
    ubuntu:18.04
```

On all hosts, install a few generally useful tools:

```bash
apt-get update
apt-get install -y curl dnsutils inetutils-ping iproute2 jq psmisc vim

ping nog.example.org
ping fso.example.org
ping storage.example.org
```
