# Cleanup
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Exit the containers.

If immutable attributes are set for some files, unset them:

```bash
docker run -it --rm --cap-add LINUX_IMMUTABLE \
    -v example.org_online:/srv/exorg_exsrv \
    ubuntu:18.04 \
    chattr -R -i /srv/exorg_exsrv
```

Remove the Docker resources:

```bash
docker rm {nog,fso,storage,ops}.example.org
docker volume rm example.org_online example.org_tape
docker network rm example.org
```
