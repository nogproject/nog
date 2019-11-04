## Authentication

The API uses a digital signature for authentication that is appended to the URL
as a query string.

The following CoffeeScript code implements the signature process:

```{.coffee}
# Encode without ':' and strip milliseconds, since they are irrelevant.
toISOStringUrlsafe = (date) -> date.toISOString().replace(/:|\.[^Z]*/g, '')

NogAuth.signRequest = (key, req) ->
  authalgorithm = 'nog-v1'
  authkeyid = key.keyid
  now = new Date()
  authdate = toISOStringUrlsafe(now)
  authexpires = config.defaultExpires
  authnonce = crypto.randomBytes(10).toString('hex')
  if urlparse(req.url).query?
    req.url += '&'
  else
    req.url += '?'
  req.url += "authalgorithm=#{authalgorithm}"
  req.url += '&' + "authkeyid=#{authkeyid}"
  req.url += '&' + "authdate=#{authdate}"
  req.url += '&' + "authexpires=#{authexpires}"
  req.url += '&' + "authnonce=#{authnonce}"

  stringToSign = req.method + "\n" + req.url + "\n"
  hmac = crypto.createHmac 'sha256', key.secretkey
  hmac.update stringToSign
  authsignature = hmac.digest 'hex'

  req.url += '&' + "authsignature=#{authsignature}"
```

The method and the whole URL path are signed.  The `authsignature` must be
appended as the last query parameter.

`authexpires` is specified in seconds.

The `authnonce` is optional.  If it is present, the request will be accepted
only once.  The `authnonce` needs to be unique only per `authdate`, so a small
nonce is usually sufficient.

`sign-req`, available from the
[nog-starter-pack](/nog/packages/files/programs/nog-starter-pack/index!0/content.tar.xz),
can be used to sign requests for curl:

Example:

```{.bash}
export NOG_KEYID=<copied>
export NOG_SECRETKEY=<copied>

curl $(
  ./tools/bin/sign-req GET \
  http://localhost:3000/api/blobs/31968d2e8b58e29e63851cb4b340216026f11f69
) | python -m json.tool
```

The following code implements the signature process in Python:

```{.python}
def sign_req(method, url):
    authkeyid = os.environ['NOG_KEYID']
    secretkey = os.environ['NOG_SECRETKEY'].encode()
    authalgorithm = 'nog-v1'
    authdate = datetime.utcnow().strftime('%Y-%m-%dT%H%M%SZ')
    authexpires = '600'
    authnonce = binascii.hexlify(os.urandom(5))

    parsed = urlparse(url)
    if parsed.query == '':
        path = parsed.path
        suffix = '?'
    else:
        path = parsed.path + '?' + parsed.query
        suffix = '&'
    suffix = suffix + 'authalgorithm=' + authalgorithm
    suffix = suffix + '&authkeyid=' + authkeyid
    suffix = suffix + '&authdate=' + authdate
    suffix = suffix + '&authexpires=' + authexpires
    suffix = suffix + '&authnonce=' + authnonce

    stringToSign = (method + '\n' + path + suffix + '\n').encode()
    authsignature = hexlify(hmac.new(
            secretkey, stringToSign, digestmod=hashlib.sha256
        ).digest()).decode()
    suffix = suffix + '&authsignature=' + authsignature
    return url + suffix
```
