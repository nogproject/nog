# Getting Software
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Building software from source

If you have a developer setup, you can build the Debian packages and Docker
images from source:

```bash
nogdir='<path-to-dev-workspace>/nog'

make -C "${nogdir}" deb docker
```

Copy the relevant Debian packages:

```bash
versionTag='...'
debs="
nog-app-2_0.1.0~dev+${versionTag}_amd64.deb
nogfsoregd_0.3.0~dev+${versionTag}_amd64.deb
nogfsoctl_0.3.0~dev+${versionTag}_amd64.deb
git-fso_0.1.0_amd64.deb
tartt_0.3.0~dev+${versionTag}_amd64.deb
nogfsostad_0.3.0~dev+${versionTag}_amd64.deb
nogfsoschd_0.3.0~dev+${versionTag}_amd64.deb
nogfsotard_0.2.0~dev+${versionTag}_amd64.deb
nogfsotarsecbakd_0.2.0~dev+${versionTag}_amd64.deb
tar-incremental-mtime_1.29.1_amd64.deb
nogfsosdwbakd3_0.2.0~dev+${versionTag}_amd64.deb
nogfsorstd_0.1.0~dev+${versionTag}_amd64.deb
nogfsodomd_0.1.0~dev+${versionTag}_amd64.deb
"

mkdir -p local/deb
cp "${nogdir}/tools/images/godev/bcpfs-perms_1.2.3_amd64.deb" 'local/deb'
for deb in $debs; do
    cp -v "${nogdir}/product/deb/${deb}" "local/deb/${deb}";
done
```

Save the Docker image versions:

```bash
nogApp2Image='nog-app-2:x.x.x...'
nogfsoregdImage='nogfsoregd:x.x.x...'
nogfsoctlImage='nogfsoctl:x.x.x...'
```

## Downloading pre-build software

If you do not have a developer setup, download pre-build Debian packages and
container images:

```bash
curl -sSLO https://visual.zib.de/2019/nog-fso-the-hard-way/latest/deb.tar
tar -xvf deb.tar
versionTag='...'
```

```bash
curl -sSLO https://visual.zib.de/2019/nog-fso-the-hard-way/latest/images.tar.bz2
bunzip2 -c images.tar.bz2 | docker load

nogApp2Image='nog-app-2:x.x.x...'
nogfsoregdImage='nogfsoregd:x.x.x...'
nogfsoctlImage='nogfsoctl:x.x.x...'
```
