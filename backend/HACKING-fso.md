# Nog Backend HACKING

## Full FSO reset

To reset the FSO state:

```bash
dc stop mongo
make gc
docker volume rm \
  nog_mongo-data \
  nog_nogfso-shadow \
  nog_nogfso-tartt \
  nog_nogfso-archive \
  nog_nogfso-tape \
  nog_nogfso-backup \
  nog_nogfso-var \
  ;

( cd apps/nog-app/meteor && echo 'db.fso.repos.drop()' | meteor mongo )
( cd apps/nog-app/meteor && echo 'db.fso.registries.drop()' | meteor mongo )
```

To reset the testing filesystems, too:

```
docker volume rm \
  nog_exinst-data \
  nog_orgfs \
  nog_orgfs2 \
  ;
```

## GitLab

Start GitLab and wait until it accepts connections.  It may take a few minutes:

```
dc up -d gitlab
dc logs -f gitlab
```

When starting GitLab the first time, open <http://localhost:10180> to set
a root password, like `test1234`.

Create an SSH key pair with empty passphrase:

```
ddev ssh-keygen -P ''
ddev cat /root/.ssh/id_rsa.pub
```

Upload the public key to GitLab <http://localhost:10180/profile/keys>.  Test
the connection:

```
ddev ssh git@localhost
```

Create an access token at
<http://localhost:10180/profile/personal_access_tokens>, and store it in the
environment for later use:

```
export NOG_TESTING_GITLAB_API_TOKEN=...

ddev bash -c "echo ${NOG_TESTING_GITLAB_API_TOKEN} >/etc/gitlab/root.token"
```

## Mongo

```
dc up -d mongo
```

## nogfso

<div class="alert alert-warning"><p>
`nogfsog2nd` has been deprecated and is not used by `nog-app` anymore.  All
GRPCs use `nogfsostad` via `nogfsoregd`.  The dev setup should work without
`nogfsog2nd`.  Consider removing nogfsog2nd-related code if it becomes tedious
to maintain it.
</p></div>

Mongo and Gitlab must be up.

Start daemons:

```
dc up -d nogfsoregd nogfsostad{,-2,-3} nogfsotar{,gct,secbak}d nogfsorstd nogfsosdwbakd3 nogfsostaudod-path-alice nogfsodomd
dc up -d nogfsotchd3  # Deprecated.
dc up -d nogfsosdwgctd{-2,-3}  # Deprecated.
dc up -d nogfsog2nd  # Deprecated.
```

Init registry, root, and repos; retry if the command fails:

```
./tools/bin/init-fso-dev
```

The script also creates directories for:

* stdrepo naming;
* orgfs service-ou naming;
* path patterns;
* help repos.

To test tartt backups:

```
./tools/bin/nogfsobak-dev
```

To test fso catalog:

```
ddev bash <<\EOF
  mkdir -p /orgfs/nog/{pub,org}/catalog &&
  if ! nogfsoctl get repos exreg | grep /example/nog/org/catalog; then
    nogfsoctl init repo --author="A U Thor <author@example.com>" exreg --no-vid /example/nog/org/catalog
  fi &&
  if ! nogfsoctl get repos exreg | grep /example/nog/pub/catalog; then
    nogfsoctl init repo --author="A U Thor <author@example.com>" exreg --no-vid /example/nog/pub/catalog
  fi &&
  echo OK
EOF

config="$(cat packages/nog-catalog-fso/tests/manual/dev_test-catalog_2017-12.yml | yq '. | tojson')" &&
json="$(printf '{"catalog": {"config": %s}}' "${config}" | jq .)" &&
printf "%s\n" "${json}"

repoId=$(nogfsoctl get repos exreg | grep /example/nog/org/catalog | cut -d '"' -f 4) && echo "${repoId}"
nogfsoctl gitnog put-path-metadata --author='a <b@c>' --message=catalog ${repoId} ".=${json}"

repoId=$(nogfsoctl get repos exreg | grep /example/nog/pub/catalog | cut -d '"' -f 4) && echo "${repoId}"
nogfsoctl gitnog put-path-metadata --author='a <b@c>' --message=catalog ${repoId} ".=${json}"
```

To test fixed suggestion metadata:

```bash
ddev bash <<\EOF
  mkdir -p /orgfs/nog/sys/fixed-md &&
  if ! nogfsoctl get repos exreg | grep /example/nog/sys/fixed-md; then
    nogfsoctl init repo --author="A U Thor <author@example.com>" exreg --no-vid /example/nog/sys/fixed-md
  fi &&
  echo OK
EOF

ddev bash ./packages/nog-suggest/tests/manual/test-fixed-md-put-path-metadata.sh
```

In a browser console:

```javascript
NogSuggest.callApplyFixedMdFromRepo({
  repo: '/example/nog/sys/fixed-md',
  mdnss: ['/sys/md/wikidata', '/sys/md/nog', '/sys/md/g/visual'],
}, console.log)

NogSuggest.callApplyFixedSuggestionNamespacesFromRepo({
  repo: '/example/nog/sys/fixed-md',
  mdnss: ['/sys/md/wikidata', '/sys/md/nog', '/sys/md/g/visual'],
  sugnss: ['/sys/sug/default', '/sys/sug/g/visual'],
}, console.log)
```

Simple scalability test:

```
ddev bash -c '
  for n in {1..10000}; do
    path="/orgfs/org/ag-alice/projects/prj-$(( ${RANDOM} % 100 ))/detail-${RANDOM}"
    echo "${path}" &&
    mkdir -p "${path}" &&
    touch "${path}/data.bin" &&
    until nogfsoctl init repo --author="a <b@c>" exreg --no-vid "/example${path}" 2>&1 | grep -q "repo already initialized at path"; do
        sleep 0.1
    done
  done
'
```

## Meteor settings

To connect `nog-app` to the nogfso daemons, configure
`apps/nog-app/meteor/_private/settings.json`:

```
{
    "fso": "dev",
    ...
}
```

To reset the FSO state:

```bash
echo 'db.fso.repos.drop()' | meteor mongo
echo 'db.fso.registries.drop()' | meteor mongo
```

Then restart Meteor.

## nogfso tests

Automated tests are in `t/`, run them with `make test-t`.  The following test
is for illustration only:

```
ddev bash -c '
  nogfsoregd --log=mu --mongodb=%2Fmongo%2Frun%2Fmongodb-27017.sock/nogfsoreg &
  while ! nc -z localhost 7550; do true; done &&
  nogfsoctl init registry exreg &&
  nogfsostad --log=mu --host=files.example.com --prefix=/example/files --prefix=/example/data --gitlab-token=/etc/gitlab/root.token exreg &
  while ! nc -z localhost 7552; do true; done &&
  nogfsoctl init root exreg --host=files.example.com --gitlab-namespace=localhost/root /example/files /usr/local &&
  ! nogfsoctl init repo --author="A U Thor <author@example.com>" exreg /invalid/path 2>/dev/null &&
  nogfsoctl init repo --author="A U Thor <author@example.com>" exreg /example/files/bin &&
  sleep 0.2 &&
  nogfsoctl init repo --author="A U Thor <author@example.com>" exreg /example/files/include &&
  true &&
  echo && echo "# registries" &&
  nogfsoctl get registries &&
  echo && echo "# info" &&
  nogfsoctl info exreg &&
  echo && echo "# registry events" &&
  nogfsoctl events registry exreg &&
  echo && echo "# roots" &&
  nogfsoctl get roots exreg &&
  echo && echo "# repos" &&
  nogfsoctl get repos exreg &&
  echo && echo "# repo" &&
  nogfsoctl get repo /example/files/bin &&
  echo && echo "# repo events" &&
  nogfsoctl events repo /example/files/bin &&
  echo &&
  echo sleeping 10s && sleep 10 &&
  nogfsoctl stat --author="A U Thor <author@example.com>" /example/files/bin &&
  nogfsoctl stat --author="A U Thor <author@example.com>" /example/files/include &&
  nogfsoctl sha --author="A U Thor <author@example.com>" /example/files/bin &&
  echo sleeping 5s && sleep 5 &&
  nogfsoctl sha --author="A U Thor <author@example.com>" /example/files/include &&
  echo sleeping 10s && sleep 10 &&
  kill -s TERM $(pgrep nogfsostad) &&
  kill -s TERM $(pgrep nogfsoregd) &&
  wait &&
  echo done
'

dc exec mongo mongo nogfsoreg --eval 'db.names.find()'
```

Testing `nogfsog2nd` (deprecated):

```
dc up -d nogfsoregd

ddev bash -c '
  nogfsog2nd --log=mu --prefix=/example exreg &
  while ! nc -z localhost 7554; do true; done &&
  nogfsoctl gitnog head /example/files/go/bin &&
  nogfsoctl gitnog summary /example/files/go/blog &&
  nogfsoctl gitnog meta /example/files/bin &&
  kill -s TERM $(pgrep nogfsog2nd) &&
  wait &&
  echo done
'
```

```bash
adminJwt="$(ddev cat /nog/jwt/tokens/admin.jwt)" && echo "${adminJwt}"

curl \
  -H "Authorization: Bearer ${adminJwt}" \
  -H 'Content-type: application/json' \
  -d '{
    "expiresIn": 600,
    "scopes": [
      { "action": "fso/read-repo", "path": "/example/files/bin" }
    ]
  }' \
  http://localhost:3000/api/v1/fso/auth \
| jq .

```
