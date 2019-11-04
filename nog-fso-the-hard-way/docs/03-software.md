# Getting Software
By Steffen Prohaska
<!--@@VERSIONINC@@-->

If you have a developer setup, you can build the programs from source:

```bash
nogdir='<path-to-dev-workspace>/nog'

mkdir -p local/release
make -C "${nogdir}" binaries nog-app-2
tar -C "${nogdir}/product" -cjvf local/release/nogfso.tar.bz2 bin
cp "${nogdir}/product/nog-app-2.tar.gz" local/release/nog-app-2.tar.gz
cp "${nogdir}/tools/images/godev/bcpfs-perms_1.2.3_amd64.deb" local/release
```

Otherwise, download and unpack a release that has been pre-build for the
tutorial:

```bash
curl -sSLO https://visual.zib.de/2019/nog-fso-the-hard-way/latest/release.tar
tar -xvf release.tar
```
