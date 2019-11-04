# Bootstrapping the File Server Vagrant VM

With the Debian packages:

```bash
echo "versionTag=${versionTag}"
find local/deb -type f
```

Configure a Vagrant VM:

```bash
 cat <<EOF >Vagrantfile
\$versionTag = "${versionTag}"
EOF
 cat <<\EOF >>Vagrantfile
$debs = [
  "bcpfs-perms_1.2.3",
  "nogfsoctl_0.3.0~dev",
  "git-fso_0.1.0",
  "tartt_0.3.0~dev",
  "nogfsostad_0.3.0~dev",
  "nogfsoschd_0.3.0~dev",
  "nogfsotard_0.2.0~dev",
  "nogfsotarsecbakd_0.2.0~dev",
  "tar-incremental-mtime_1.29.1",
  "nogfsosdwbakd3_0.2.0~dev",
  "nogfsorstd_0.1.0~dev",
  "nogfsodomd_0.1.0~dev",
]

Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/bionic64"

  $debs.each do |stem|
    if stem.end_with?("~dev") then
      deb = "#{stem}+#{$versionTag}_amd64.deb"
    else
      deb = "#{stem}_amd64.deb"
    end
    src = File.join("local/deb", deb)
    dst = File.join("${HOME}", deb)
    config.vm.provision "file", source: src, destination: dst
  end

  config.vm.define "storage" do |storage|
    storage.vm.hostname = "storage"
  end
end
EOF
```

Create the Vagrant VM:

```bash
vagrant up
```

Create system users and configure the Debian packages:

```bash
 vagrant ssh storage -- sudo bash -sx -o errexit -o nounset <<\EOF
addgroup --system ngfsta
addgroup --system ngftar
addgroup --system ngfbak
adduser --system --home /nonexistent --no-create-home --ingroup ngfsta ngfsta
adduser --system --home /nonexistent --no-create-home --ingroup ngftar ngftar
adduser --system --home /nonexistent --no-create-home --ingroup ngfsta ngfrst
adduser ngfrst ngftar
adduser --system --home /nonexistent --no-create-home --ingroup ngfbak ngfbak
addgroup --system ngfdom
adduser --system --home /nonexistent --no-create-home --ingroup ngfdom ngfdom

echo 'nogfsostad nogfsostad/user string ngfsta' | debconf-set-selections
echo 'nogfsostad nogfsostad/org_group string ag_exorg' | debconf-set-selections
echo 'nogfsotard nogfsotard/nogfsostad_user string ngfsta' | debconf-set-selections
echo 'nogfsotard nogfsotard/nogfsotard_user string ngftar' | debconf-set-selections
echo 'nogfsosdwbakd3 nogfsosdwbakd3/nogfsostad_user string ngfsta' | debconf-set-selections
echo 'nogfsosdwbakd3 nogfsosdwbakd3/nogfsosdwbakd3_user string ngfbak' | debconf-set-selections
echo 'nogfsorstd nogfsorstd/user string ngfrst' | debconf-set-selections
echo 'nogfsorstd nogfsorstd/group string ngfsta' | debconf-set-selections
echo 'nogfsodomd nogfsodomd/user string ngfdom' | debconf-set-selections
EOF
```

Install the Debian packages:

```bash
 vagrant ssh storage -- sudo bash -sx -o errexit -o nounset <<\EOF
chmod a+r *.deb
apt-get update
apt-get install -y $PWD/tar-incremental-mtime_*.deb
apt-get install -y $PWD/*.deb
rm /usr/share/git-core/templates/hooks/*.sample
EOF
```

Create example groups and users:

```bash
 vagrant ssh storage -- sudo bash -sx -o errexit -o nounset <<\EOF
groupAgSuper='ag_exorg'

orgUnits='
ag-alice
ag-bob
ag-charly
em-facility
lm-facility
ms-facility
'

services='
spim-100
spim-222
tem-505
rem-707
ms-data
'

facilities='
em
lm
ms
'

# Lines: <user> <orgUnit> <services>...
users='
alice  ag-alice  rem-707 tem-505
bob    ag-bob    rem-707 tem-505
charly ag-charly rem-707 tem-505
'

addgroup "${groupAgSuper}"

for ou in ${orgUnits}; do
    addgroup "exorg_${ou}"
    adduser --system --shell /bin/bash --ingroup "exorg_${ou}" "${ou}-user"
    adduser "${ou}-user" "${groupAgSuper}"
done

for d in ${services}; do
    addgroup "exsrv_${d}"
done

for f in ${facilities}; do
    addgroup "exsrv_${f}-ops"
done

grep -v '^ *$' <<<"${users}" | while read -r user ou srvs; do
    adduser --system --shell /bin/bash --ingroup "exorg_${ou}" "${user}"
    adduser "${user}" "${groupAgSuper}"
    for s in ${srvs}; do
        adduser "${user}" "exsrv_${s}"
        echo "Added user \`${user}\` to service \`${s}\`."
    done
done
EOF
```

Forward the Kubernetes services to the Vargant VM:

In a separate terminal, start and keep running:

```bash
kubectl port-forward services/fso 7550:7550 7551:7551
```

In another separate terminal, start and keep running:

```bash
vagrant ssh storage -- -R 7550:localhost:7550 -R 7551:localhost:7551
echo '127.0.0.1 fso.example.org' | sudo tee -a /etc/hosts

nc -z fso.example.org 7550 && echo ok
nc -z fso.example.org 7551 && echo ok
```
