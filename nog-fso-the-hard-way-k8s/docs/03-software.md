# Getting Software
By Steffen Prohaska
<!--@@VERSIONINC@@-->

If you have a developer setup, you can build the programs and container images
from source:

```bash
nogdir='<path-to-dev-workspace>/nog'

make -C "${nogdir}" docker

nogApp2Image='nog-app-2:x.x.x...'
nogfsoregdImage='nogfsoregd:x.x.x...'
nogfsoctlImage='nogfsoctl:x.x.x...'
nogfsostoImage='nogfsosto:x.x.x...'
```

Otherwise, download pre-build container images:

```bash
curl -sSLO https://visual.zib.de/2019/nog-fso-the-hard-way/latest/images.tar.bz2
bunzip2 -c images.tar.bz2 | docker load

nogApp2Image='nog-app-2:x.x.x...'
nogfsoregdImage='nogfsoregd:x.x.x...'
nogfsoctlImage='nogfsoctl:x.x.x...'
nogfsostoImage='nogfsosto:x.x.x...'
```
