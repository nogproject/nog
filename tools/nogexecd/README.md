# Nog job execution daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Introduction

This directory contains the nog execution daemon.  It connects to a nog app,
polls for jobs, and executes them locally or using Slurm.

## Running nogexecd for job submission to slurm

Configure the environment in `NOGJOBD_CONFIG`.  Explicitly specify https and
the SSL port 443.  Set `worker` to use slurm job submission.  See
`nogjobd.coffee` for alternative options.

```bash
export NOGJOBD_CONFIG='{
    "NOG_URL": "https://<nog-url>:443",
    "worker": "./nogexecd-slurm"
}'
```

The configuration above should work with `ddp@0.12.0`, as configured in
`package.json`.  `ddp@0.11.0` required the direct URL after resolving a CNAME,
which can be done for example with `dig nog.zib.de CNAME +short`.

```bash
export NOGJOBD_CONFIG='{
    "NOG_URL": "https://<resolved-cname>:443",
    "worker": "./nogexecd-slurm"
}'
```

Configure a cache path (the directory must exist):

```bash
export NOG_CACHE_PATH=/some/local/path
```

Configure the API URL and an access key for a `nogexecbot*` (get it from the
nog admin UI):

```bash
export NOG_API_URL=https://<nog-url>/api
export NOG_KEYID=<get-from-web-admin-ui>
export NOG_SECRETKEY=<get-from-web-admin-ui>
```

Start the daemon via watchdog:

```bash
cd tools/nogexecd
export PATH=$(pwd)/node/latest/bin:$PATH  # See setup below.
./node_modules/.bin/coffee nogjobd-forever
```

## Setup on Linux

Create a Python 3 virtualenv and install dependencies:

```bash
cd tools/nogexecd
virtualenv -p python3 virtualenv
./virtualenv/bin/pip install -r requirements.txt
```

Install node:

```bash
mkdir node
cd node
version=6.11.1
curl -O https://nodejs.org/dist/v${version}/node-v${version}-linux-x64.tar.gz
tar xvf node-v${version}-linux-x64.tar.gz
ln -s node-v${version}-linux-x64 latest
cd ..

export PATH=$(pwd)/node/latest/bin:$PATH
```

Install npm dependencies:

```bash
npm install
```

## Local Testing

The daemon can be run locally without slurm, executing jobs in a subprocess
(see `nogexecd-subprocess.coffee`):

```bash
export NOG_API_URL=http://localhost:3000/api
export NOG_CACHE_PATH=<right-path-for-your-local-setup>
export NOG_KEYID=<get-from-web-admin-ui-at-localhost>
export NOG_SECRETKEY=<get-from-web-admin-ui-at-localhost>
./nogjobd-forever
```
