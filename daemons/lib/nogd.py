from boto3.session import Session as AwsSession
from botocore.client import Config as AwsConfig
from botocore.credentials import ReadOnlyCredentials
from botocore.session import get_session as get_botocore_session
from copy import copy, deepcopy
from datetime import datetime, timedelta
from os import environ
from os.path import expanduser
from pymongo import MongoClient
from signal import signal, SIGTERM
from threading import Thread, Lock
from time import sleep, time, gmtime
import attr
import boto3
import hvac
import json
import logging
import prometheus_client as prom

logger = logging.getLogger('nogd')


LOCK_EXPIRE_TIMEDELTA = timedelta(minutes=5)
LOCK_RENEW_INTERVAL_S = 60


class BlobStateError(Exception):
    """`BlobStateError` indicates inconsistent blob-related state.

    It should be used to indicate serious problems such as a mismatch between
    the blob size stored in MongoDB and the object size returned by S3.

    """
    pass


class SigTerm(Exception):
    pass


def installSigTermHandler():

    def handler(sig, stack):
        raise SigTerm()

    signal(SIGTERM, handler)


# The logging level is only controlled on the root logger.  All other loggers
# pass their messages up the logging tree.  Python code for configuring logging
# turned out to be simpler and more robust than `logging.config.dictConfig()`.

def configureLogging(cfg):
    """`configureLogging()` configures Python `logging`.

    `cfg` is a dict that may contain `loglevel`.  The default is `INFO`.

    """
    formatter = logging.Formatter(
        fmt='{asctime} - {levelname:8} - {name:11}: {message}', style='{',
    )
    formatter.converter = gmtime  # Use UTC.

    console = logging.StreamHandler()
    console.setLevel(logging.NOTSET)
    console.setFormatter(formatter)

    logging.root.addHandler(console)
    logging.root.setLevel(cfg.get('loglevel', logging.INFO))


# See Prometheus doc 'Metric and label naming'
# <https://prometheus.io/docs/practices/naming/> for general naming strategy.

nogdBlobsProcessedTotal = prom.Counter(
    'nogd_blobs_processed_total',
    (
        'Total number of processed blobs, '
        'including blobs that were already up-to-date.'
    ),
)

nogdBlobsReadBytesTotal = prom.Counter(
    'nogd_blobs_read_bytes_total',
    'Cumulative size of blobs read during processing.',
    ['purpose', 'bucket'],
)

nogdStaleLocksExpiredTotal = prom.Counter(
    'nogd_stale_locks_expired_total',
    'Total number of stale locks that were expired.',
    ['op'],
)


def startMetricsServer(port=8081, addr=''):
    """`startMetricsServer()` runs a Prometheus metrics endpoint.

    """
    prom.start_http_server(port, addr)
    msg = 'Prometheus metrics server listening on {}:{}'.format(addr, port)
    logger.info(msg)


class Vault:
    """`Vault` manages a config dict with secrets from Vault.

    See `newMongoBucketsCfgVault()` for an example how to instantiate `Vault`.
    A config dict is passed to the constructor `vault = Vault(cfg)`.  Keys that
    should be managed by Vault are then declared by `vault.leaseTo(vaultPath,
    destDict, keymap)`, where `destDict` is either `cfg` or a subdict of `cfg`.
    `vaultPath` is a path that is read from Vault.  The result fields are
    stored into `destDict` after renaming keys as indicated by the `keymap`.

    `vault.startRenewalDaemon()` starts background renewal of secret leases.
    `vault.shutdown()` should be called right before exiting the application to
    give `Vault` a chance to revoke secrets.

    Use `vault.cfg` to read the configuration with the current secrets.  Do not
    use the original dict `cfg` to access the current secrets, since it is not
    thread-safe and may yield inconsistent secrets, like mismatching access key
    id and secret access key.

    `vault.mtime` changes whenever a secret has been renewed.  It can be used
    to detect such changes and reconfigure the application if necessary, like
    reestablishing connections with the updated secrets.

    """
    def __init__(self, cfg):
        self._lock = Lock()
        self._leaseLock = Lock()
        self._cfg = cfg
        self._updateReadView()
        self._client = None
        self._forceRead = False
        self._leases = {}
        self._leaving = False

    @property
    def cfg(self):
        with self._lock:
            return self._readCfg

    @property
    def mtime(self):
        with self._lock:
            return self._readCfg['mtime']

    def _updateReadView(self):
        self._cfg['mtime'] = time()
        cfg = deepcopy(self._cfg)
        with self._lock:
            self._readCfg = cfg

    # The general Vault connection setup is performed once during
    # `_connectVault()`.
    #
    # The token is reread from disk during each tick to support token
    # pick up new tokens that have been replacement by a separate background
    # task.  If the token changes, all leases are renewed.  The old token is
    # probably going to be revoked soon and all old leases with it.

    def _connectVault(self):
        url = self._cfg['vaultAddr']
        cacert = self._cfg.get('vaultCacert', None)
        with open(expanduser('~/.vault-token'), 'r') as fp:
            token = fp.read()
        if cacert:
            msg = "Begin connecting to Vault at '{}' with CA cert '{}'."
            logger.info(msg.format(url, cacert))
            self._client = hvac.Client(url, verify=cacert, token=token)
        else:
            msg = "Begin connecting to Vault at '{}'."
            logger.info(msg.format(url))
            self._client = hvac.Client(url, token=token)
        logger.info('Established connection to Vault.')

    def _replaceToken(self):
        with open(expanduser('~/.vault-token'), 'r') as fp:
            token = fp.read()
        if token == self._client.token:
            return False
        self._client.token = token
        logger.info('Replaced Vault token.')
        return True

    # XXX AWS STS tokens cannot be renewed, although the token reported
    # `renewable=true` until Vault 0.6.1; see Vault source
    # `.../aws/secret_access_keys.go` <https://goo.gl/kBeNYL>.  The issues
    # should be fixed with Vault 0.6.2 (unreleased); see
    # <https://github.com/hashicorp/vault/issues/1800>.
    #
    # As a workaround, detect STS tokens based on their lease id path and force
    # `renewable=false`.  The workaround can be removed when we use a new
    # enough version of Vault everywhere.

    def _readVault(self, path):
        if not self._client:
            self._connectVault()
        res = self._client.read(path)
        if '/sts/' in res['lease_id']:
            res['renewable'] = False
        res['rtime'] = time()
        # res['lease_duration'] = 1810  # Can be useful for debugging.
        return res

    def _renew_secret(self, lease_id):
        res = self._client.renew_secret(lease_id)
        res['rtime'] = time()
        return res

    def _revoke_secret(self, lease_id):
        self._client.revoke_secret(lease_id)

    # `leaseTo()` reuses previously read Vault paths if necessary to minimize
    # the number of Vault reads.

    def leaseTo(self, path, dest, keymap):
        if path.startswith('vault:'):
            path = path.split(':')[1]
        try:
            lease = self._leases[path]
        except KeyError:
            lease = {
                'vault': self._readVault(path),
                'targets': []
            }
            self._leases[path] = lease
            logger.info('Read Vault secret `{}`'.format(path))
        lease['targets'].append({'dest': dest, 'keymap': keymap})
        for src, dst in keymap.items():
            dat = lease['vault']['data'].get(src, None)
            if dat:
                dest[dst] = dat
            else:
                dest.pop(dst, None)  # `del dest[dst]` ignoring missing key.
        self._updateReadView()

    # Refresh leases 30m before they expire.  If renewal fails or returns a
    # short duration, fall back to reread.
    #
    # A token replacement forces a reread of all leases, because the old token
    # is probably going to be revoked soon.  A forced read will be retried
    # until all leases have been successfully updated.
    #
    # `tick()` should not be called too frequently to avoid rapid retries after
    # Vault errors.  One tick per minute seems reasonable.  If renew repeatedly
    # fails, there will be around 30 retries before the lease expires.

    def _tick(self):
        self._forceRead |= self._replaceToken()

        for path, lease in self._leases.items():
            now = time()

            if not self._forceRead:
                rtime = lease['vault']['rtime']
                duration = lease['vault']['lease_duration']
                if (rtime + duration - 1800) > now:
                    continue

            with self._leaseLock:
                if self._leaving:
                    return

                if self._forceRead:
                    pass
                elif lease['vault']['renewable']:
                    lid = lease['vault']['lease_id']
                    try:
                        lease['vault'] = self._renew_secret(lid)
                        logger.info('Renewed Vault lease `{}`'.format(lid))
                        shouldRead = False
                    except Exception as err:
                        msg = (
                            'Failed to renew Vault lease {}: {}; ' +
                            'falling back to reread.'
                        )
                        logger.warning(msg.format(lid, err))
                        shouldRead = True
                    if not shouldRead:
                        rtime = lease['vault']['rtime']
                        duration = lease['vault']['lease_duration']
                        if (rtime + duration - 2700) < now:
                            logger.warning(
                                'Renewed Vault lease has a short duration; ' +
                                'falling back to reread.'
                            )
                            shouldRead = true
                else:
                    shouldRead = True

                if self._forceRead or shouldRead:
                    try:
                        lease['vault'] = self._readVault(path)
                        logger.info('Reread Vault secret `{}`'.format(path))
                    except Exception as err:
                        msg = 'Failed to reread Vault secret {}: {}'
                        logger.error(msg.format(path, err))
                        continue

            for target in lease['targets']:
                for src, dst in target['keymap'].items():
                    dat = lease['vault']['data'].get(src, None)
                    if dat:
                        target['dest'][dst] = dat
                    else:
                        target['dest'].pop(dst, None)
            self._updateReadView()

        if self._forceRead:
            msg = 'Completed rereading leases with replaced Vault token.'
            logger.info(msg)
        self._forceRead = False

    def startRenewalDaemon(self):
        if len(self._leases) == 0:
            logger.info('Disabled Vault: no leases.')
            return

        def run():
            while True:
                self._tick()
                sleep(60)

        t = Thread(name='VaultDaemon', target=run, daemon=True)
        t.start()
        logger.info('Started background Vault lease renewal.')

    def shutdown(self):
        with self._leaseLock:
            self._leaving = True
            for path, lease in self._leases.items():
                lid = lease['vault']['lease_id']
                if lid:
                    self._revoke_secret(lid)
                    logger.info('Revoked Vault lease `{}`'.format(lid))


def updateConfFromEnv(cfg, envvar):
    envConf = environ.get(envvar, None)
    if envConf and envConf.startswith('{'):
        cfg.update(json.loads(envConf))
    else:
        with open(envConf, 'r') as fp:
            cfg.update(json.load(fp))


def hideSecret(k, v):
    whitelist = (
        'awsAccessKeyId',
        'awsRegion',
        'buckets',
        'daemonId',
        'endpointUrl',
        'keyVault',
        'loglevel',
        'multiPartDefaultsAlgo',
        'name',
        'nogMongoCa',
        'nogMongoCert',
        'readBuckets',
        'resetCutoff',
        'signatureVersion',
        'sourceBuckets',
        'vaultAddr',
        'vaultCacert',
    )
    if k in whitelist:
        return copyWithoutSecrets(v)
    else:
        return '**********'


def copyWithoutSecrets(cfg):
    if isinstance(cfg, list):
        return [copyWithoutSecrets(v) for v in cfg]
    elif isinstance(cfg, dict):
        return {k: hideSecret(k, v) for k, v in cfg.items()}
    else:
        return cfg


def newMongoBucketsCfgVault(cfg):
    vault = Vault(cfg)
    if cfg['nogMongoUrl'].startswith('vault:'):
        vault.leaseTo(cfg['nogMongoUrl'], cfg, keymap={ 'url': 'nogMongoUrl' })

    for v in cfg['buckets']:
        keyVault = v.get('keyVault', None)
        if not keyVault:
            continue
        vault.leaseTo(
            keyVault, v,
            keymap={
                'access_key': 'awsAccessKeyId',
                'secret_key': 'awsSecretAccessKey',
                'security_token': 'awsSessionToken',
            },
        )

    return vault


@attr.s
class MongoBucketContext(object):
    """`MongoBucketContext` is used to pass around runtime dependencies.

    The context is initialized by calling `newMongoBucketContext()` with a
    config dict.

    The context attributes are:

    - `daemonId`: A string that identifies the application instance.  It is
      used as an `_id` in the MongoDB `daemons` collection.
    - `db`: A MongoDB connection.
    - `s3Clients`: A dict with one AWS S3 client instance for each configured
      bucket.
    - `sourceBuckets`: A list of bucket names used as replication sources.
    - `desiredBuckets`: A list of bucket names that are the replication
      targets.
    - `readBuckets`: A list of buckets that are used as data sources when
      computing checksums.

    """
    daemonId = attr.ib()
    db = attr.ib()
    s3Clients = attr.ib()
    sourceBuckets = attr.ib()
    readBuckets = attr.ib()
    desiredBuckets = attr.ib()

    def checkBucketAccess(self):
        for bucket, client in self.s3Clients.items():
            logger.info('Begin checking access to bucket `{}`.'.format(bucket))
            client.head_bucket(Bucket=bucket)
            logger.info('Checked access: HEAD {} ok.'.format(bucket))


def getpath(data, path):
    """`getpath()` returns the value at a dot-separated getitem path.

    `data` can be a nested combination of dicts and arrays.  Each part of the
    path is interpreted as an integer if it looks like an integer.

    """
    for p in path.split('.'):
        try:
            p = int(p)
        except ValueError:
            pass
        data = data[p]
    return data


class VaultAwsCredentials(object):
    """`VaultAwsCredentials` provide AWS credentials managed by `vault`.

    `VaultAwsCredentials` implements the `botocore.credentials.Credentials`
    interface, returning the latest config from `vault` on each access, so that
    background token renewal works for long-running operations.

    Use `newVaultAwsSession()` to instantiate a Boto3 session that uses
    `VaultAwsCredentials`.

    """
    def __init__(self, vault, cfgpath):
        self.vault = vault
        self.cfgpath = cfgpath
        self.method = 'vault'

    @property
    def access_key(self):
        return self.get_frozen_credentials().access_key

    @property
    def secret_key(self):
        return self.get_frozen_credentials().secret_key

    @property
    def token(self):
        return self.get_frozen_credentials().token

    def get_frozen_credentials(self):
        cfg = getpath(self.vault.cfg, self.cfgpath)
        return ReadOnlyCredentials(
            cfg['awsAccessKeyId'],
            cfg['awsSecretAccessKey'],
            cfg.get('awsSessionToken', None),
        )


def newVaultAwsSession(vault, cfgpath):
    """`newVaultAwsSession()` returns a session with credentials from `vault`.

    See <https://gist.github.com/kapilt/ac8e222081f63ba64e93> for the general
    idea how to tweak the Boto3 internals.

    """
    s = get_botocore_session()
    s._credentials = VaultAwsCredentials(vault, cfgpath)
    return AwsSession(botocore_session=s)


# See <https://api.mongodb.com/python/current/examples/tls.html>
def newMongoClient(cfg):
    kwargs = {'host': cfg['nogMongoUrl']}

    ca = cfg.get('nogMongoCa')
    if ca:
        kwargs['ssl'] = True
        kwargs['ssl_ca_certs'] = ca

    cert = cfg.get('nogMongoCert')
    if cert:
        kwargs['ssl'] = True
        kwargs['ssl_certfile'] = cert

    return MongoClient(**kwargs)


def newMongoBucketContext(vault, required):
    """`newMongoBucketContext()` instantiates dependencies.

    `newMongoBucketContext()` instantiates a MongoDB connection and AWS S3
    clients as specified in the configuration and returns them as a
    `MongoBucketContext`.

    `required` is a list of otherwise optional keys whose presence is checked
    in `vault.cfg`.

    Supported bucket configs:

    - AWS eu-central-1:  Set `awsRegion=eu-central-1' to automatically use S3
      v4 signatures.
    - AWS other: Set `awsRegion=...` to use the automatic S3 endpoints with
      host addressing, v2 signatures.
    - Ceph S3: Set `endpointUrl=...` to use a non-AWS endpoint with path
      addressing, v2 signatures.

    AWS credentials are expected in `awsAccessKeyId` and `awsSecretAccessKey`
    with optional `awsSessionToken` for STS tokens.  The latest credentials as
    managed by `vault` are used on each API access, so that STS token renewal
    works as expected during long-running copy operations.

    """
    cfg = vault.cfg

    for rq in required:
        if rq not in cfg:
            raise ValueError('Missing config key `{}`.'.format(rq))

    daemonId = cfg['daemonId']
    mongo = newMongoClient(cfg)
    db = mongo.get_default_database()

    s3Clients = {}
    for idx, bCfg in enumerate(cfg['buckets']):
        kwargs = {}
        awsRegion = bCfg.get('awsRegion', None)
        if awsRegion:
            kwargs['region_name'] = awsRegion
            if awsRegion == 'eu-central-1':
                kwargs['config'] = AwsConfig(signature_version='s3v4')
            else:
                kwargs['config'] = AwsConfig()
        else:
            kwargs['endpoint_url'] = bCfg['endpointUrl']
            # Use old v2 signatures if unspecified to work with Ceph S3.  Do
            # not rely on boto's default, because it has changed in
            # botocore-1.5.71; see
            # <https://github.com/boto/botocore/blob/develop/CHANGELOG.rst#1571>.
            sigV = bCfg.get('signatureVersion', 'v2')
            if sigV == 'v2':
                signature_version = 's3'
            elif sigV == 'v4':
                signature_version = 's3v4'
            else:
                msg = 'Invalid `signatureVersion`; must be `v2` or `v4`.'
                raise ValueError(msg)
            kwargs['config'] = AwsConfig(
                signature_version=signature_version,
                s3={'addressing_style': 'path'},
            )
        session = newVaultAwsSession(vault, 'buckets.{}'.format(idx))
        client = session.client(service_name='s3', **kwargs)
        s3Clients[bCfg['name']] = client

    return MongoBucketContext(
        daemonId=daemonId,
        db=db,
        s3Clients=s3Clients,
        sourceBuckets=cfg.get('sourceBuckets', None),
        readBuckets=cfg.get('readBuckets', None),
        desiredBuckets=cfg.get('desiredBuckets', None),
    )


def ensureMtime(blob, ctx):
    """`ensureMtime()` adds `mtime` to the blob if it is missing.

    `ensureMtime()` can be used to upgrade legacy MongoDB entries, which were
    created before `nog-blob` added `mtime`.

    It is safe to call `ensureMtime()` concurrently.  If so, `mtime` might be
    updated multiple times with a Mongo server timestamp.  The largest
    timestamp will be finally recorded.

    """
    if 'mtime' in blob:
        return
    bid = blob['_id']
    ur = ctx.db.blobs.update_one(
        { '_id': bid },
        { '$currentDate': { 'mtime': True } },
    )
    if ur.matched_count == 1:
        logger.info('Added mtime to blob {}.'.format(bid))
    else:
        logger.error('Failed to add mtime to blob {}.'.format(bid))


class DocLock(object):
    """`DocLock` is used to maintain a lock on a MongoDB doc.

    Locks are maintained in the array `doc.locks`.  See `nogreplicad` and
    `nogsumd` for usage.

    """
    def __init__(self, collection, docid, holder, core):
        self.collection = collection
        self.docid = docid
        self.holder = holder
        self.core = core

    def tryLock(self):
        minLock = self.core
        fullLock = copy(self.core)
        fullLock.update({
            'ts': datetime.utcnow(),
            'holder': self.holder,
        })
        ur = self.collection.update_one(
            {
                '_id': self.docid,
                'locks': { '$not': { '$elemMatch': minLock } },
            },
            { '$push': { 'locks': fullLock } }
        )
        if ur.modified_count != 1:
            return False
        logger.info('Locked {}.'.format(self.lockstr()))
        self.nextRenewal = time() + LOCK_RENEW_INTERVAL_S
        return True

    def unlock(self):
        lockSel = copy(self.core)
        lockSel.update({ 'holder': self.holder })
        ur = self.collection.update_one(
            { '_id': self.docid },
            { '$pull': { 'locks': lockSel } },
        )
        if ur.modified_count == 1:
            logger.info('Unlocked {}.'.format(self.lockstr()))

    def lockstr(self):
        core = [
            '{}={}'.format(k, self.core[k])
            for k in sorted(self.core.keys())
        ]
        return '{}/{}'.format(self.docid, ','.join(core))

    def renewLock(self):
        if time() < self.nextRenewal:
            return

        self.nextRenewal = time() + LOCK_RENEW_INTERVAL_S

        lockSel = copy(self.core)
        lockSel.update({ 'holder': self.holder })
        ur = self.collection.update_one(
            { '_id': self.docid, 'locks': { '$elemMatch': lockSel } },
            { '$currentDate': { 'locks.$.ts': True } }
        )
        if ur.modified_count == 1:
            logger.info('Renewed lock {}.'.format(self.lockstr()))
        else:
            logger.error('Failed to renew lock {}.'.format(self.lockstr()))


# The `debug*()` functions below may be useful for debuggin.  They are not used
# in production.

def debugPrintBlob(blob, ctx):
    bid = blob['_id']
    print('DEBUG blob:', ctx.db.blobs.find_one({ '_id': bid }))


def debugUnsetLocs(blob, ctx):
    bid = blob['_id']
    ctx.db.blobs.update_one(
        { '_id': bid },
        {
            '$currentDate': { 'mtime': True },
            '$unset': { 'locs': '' }
        },
    )


def debugClearAllFutures(ctx):
    logger.debug('Clearing all futures.')
    ctx.db.blobs.update_many(
        {},
        { '$unset': { 'futs': ''} },
    )


def debugClearAllLocks(ctx):
    logger.debug('Clearing all locks.')
    ctx.db.blobs.update_many(
        {},
        { '$unset': { 'locks': ''} },
    )


def debugClearAllErrors(ctx):
    logger.debug('Clearing all errors.')
    ctx.db.blobs.update_many(
        {},
        { '$unset': { 'errors': ''} },
    )


def debugClearToplevelVerified(ctx):
    logger.debug('Clearing all toplevel verified.')
    ctx.db.blobs.update_many(
        {},
        { '$unset': { 'verified': ''} },
    )


def debugUnsetAllLocsLogVerified(ctx):
    logger.debug('Clearing all locs, log, and toplevel verified.')
    ctx.db.blobs.update_many(
        {},
        {
            '$unset': {
                'locs': '',
                'log': '',
                'verified': ''
            }
        },
    )


def debugSetAllMissing(bucket, ctx):
    ctx.db.blobs.update_many(
        { 'locs.bucket': bucket },
        {
            '$set': { 'locs.$.status': 'missing' },
            '$currentDate': { 'mtime': True },
        },
    )
