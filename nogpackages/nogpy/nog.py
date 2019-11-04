#!/usr/bin/env python3

"""
The package nog.py wraps the nog REST API to provide a convenient Python
interface, including caching of immutable content.  See README.md for an
overview.
"""

from __future__ import unicode_literals
from binascii import hexlify
from concurrent.futures import ThreadPoolExecutor, as_completed
from copy import copy, deepcopy
from datetime import datetime
from dateutil.parser import parse as dateparse
from fnmatch import fnmatch
from requests.adapters import HTTPAdapter
from tempfile import NamedTemporaryFile
import hashlib
import hmac
import json
import sys
import os
import os.path
import re
import requests
import shutil
import stat
import six
if sys.version_info.major == 3:
    from urllib.parse import urlparse
else:
    from urlparse import urlparse


NULL_SHA1 = u'0000000000000000000000000000000000000000'
contentTypeJSON = {'Content-Type': 'application/json'}

# Number of parallel uploads to S3.
S3_NPARALLEL = 32

# `POST_BUFFER_SIZE` is the batch size for small POSTs.
# `POST_BUFFER_SIZE_LIMIT` is the max size of an individual POST. See
# `README.md` for details.
POST_BUFFER_SIZE = 10000
POST_BUFFER_SIZE_LIMIT = 200000

verbosity = 0

# `NOG_ERRATA` controls how to handle errata:
#
#  - `error`: raise `ErrataError`.
#  - `warning`: report warning and continue.
#  - `ignore`: silently ignore.
#
# Errata are only handled right after GET from the API.  Errata are stripped
# before adding the content to the local cache.  Cached content will be used as
# stored, without rechecking errata.

errataPolicy = os.environ.get('NOG_ERRATA', 'error')


# Use a connect timeout slightly larger than 3 as recommended in
# <http://www.python-requests.org/en/latest/user/advanced/#timeouts>.
#
# Retry a few times by default to hide the connection problems that we have
# observed with modulus.
#
# A short timeout with a few retries seems to work robustly in practice for
# syncing repos to vsl4.  The read timeout (the second entry) should be larger
# than the timeout in nog-rest, so that nog-rest reports an timeout error
# before nogpy retries, which would cause an invalid nonce error.
#
# The `putS3Timeout` is larger to avoid problems with uploading large files.
# We've observed errors 'the write operation timed out' when we used
# `requestTimeout` with PUT via a reverse proxy to Ceph S3 over DSL.  The
# timeout should be limited, so that real errors get reported after some time.
# We should consider keeping the timeout reasonably small and adding an
# environment variable as an option to configure larger timeouts if it turns
# out that much larger timeouts are necessary for slow connections.

requestTimeout = (3.1, 27)
putS3Timeout = 300
max_retries = int(os.environ.get('NOG_MAX_RETRIES', 5))


def _apiUrl():
    base = os.environ.get('NOG_API_URL', 'http://localhost:3000/api')
    return base + '/v1'


session = requests.Session()
session.mount(_apiUrl(), HTTPAdapter(max_retries=max_retries))


def _printmsg(level, msg):
    if level <= verbosity:
        print(msg)


class ErrataError(Exception):
    pass


class EtagError(Exception):
    pass


# `_handleErrata()` handles and strips errata, so that the main code paths can
# be unaware of errata.

def _handleErrata(content, ty):
    errata = content.get('errata', None)
    if not errata:
        return

    del content['errata']

    if errataPolicy == 'ignore':
        return

    codes = ', '.join(era['code'] for era in errata)
    s = 's'
    if len(errata) == 1:
        s = ''
    msg = '{ty} {id} has been marked with errata code{s} {codes}.'.format(
        ty=ty, id=content['_id'], s=s, codes=codes
    )

    if errataPolicy == 'warning':
        _printmsg(0, 'Warning: {}'.format(msg))
        return

    raise ErrataError(msg)


def openRepo(name):
    name = _completeRepoName(name)
    remote = RemoteRepo(_apiUrl() + '/repos/' + name)
    remote.getMaster()
    return remote


def createRepo(name):
    """
    Creates a new repository and adds an initial commit.
    """
    name = _completeRepoName(name)
    url = _apiUrl() + '/repos'
    url = sign_req('POST', url)
    content = {
        'repoFullName': name
    }
    res = session.post(
            url, headers=contentTypeJSON, data=stringify(content),
            timeout=requestTimeout)
    _checkStatus(res, 201)

    remoteRepo = openRepo(name)
    tree = Tree()
    remoteRepo.createInitialCommit("Initial commit", tree)

    return remoteRepo


def _completeRepoName(name):
    if '/' in name:
        return name
    try:
        username = os.environ['NOG_USERNAME']
    except KeyError:
        raise RuntimeError(_denl("""
                Failed to construct repo name: name `{0}` looks like a
                short name, which requires the environment variable
                `NOG_USERNAME` to construct a full name; but
                `NOG_USERNAME` is unset.
            """).format(name))
    return username + '/' + name


# The entry cache `_entryCache` can be shared between repos.  If an entry has
# been cached from any repo, the cache entry can be used to get the entry from
# any other repo.  It effectively disables the repo sets check during get for
# locally known entries.
#
# `_knownEntryIds`, however, must not be shared between repos.  It is a local
# subset of the repo sets.  An entry is added only if the remote server
# confirmed that it is in the specific repo.
class RemoteRepo:
    def __init__(self, url, entryCache=None, blobCache=None):
        self._entryCache = entryCache or EntryCache()
        self._blobCache = blobCache or BlobCache()
        self.url = url
        self._knownEntryIds = set()
        self._knownBlobs = set()

    @property
    def fullName(self):
        # The last two parts.
        return '/'.join(self.url.split('/')[-2:])

    def getMaster(self):
        return _createEntry(
                content={
                    'type': 'commit',
                    'sha1': self.getRef('branches/master')["sha1"]
                },
                repo=self
            )

    def getRef(self, refName):
        res = self._get('db/refs/{0}'.format(refName))
        return res["entry"]

    def getObject(self, sha1):
        return _createEntry({'type': 'object', 'sha1': sha1}, self)

    def getTree(self, sha1):
        return _createEntry({'type': 'tree', 'sha1': sha1}, self)

    def getCommit(self, sha1):
        return _createEntry({'type': 'commit', 'sha1': sha1}, self)

    def createPostStream(self):
        return PostStream(self)

    def postTree(self, tree):
        with self.createPostStream() as s:
            return s.postTree(tree)

    def postObject(self, obj):
        with self.createPostStream() as s:
            return s.postObject(obj)

    def commitTree(self, subject, tree, parent, message=None, meta=None):
        return self._commitTree(subject, tree, [parent], oldCommit=None,
                                message=message, meta=meta)

    def createInitialCommit(self, subject, tree, message=None, meta=None):
        return self._commitTree(subject, tree, parents=[],
                                oldCommit=NULL_SHA1, message=message,
                                meta=meta)

    def _commitTree(self, subject, tree, parents, oldCommit=None, message=None,
                    meta=None):
        message = message or ''
        meta = meta or {}
        oldCommit = oldCommit or parents[0]
        if not isSha1(tree):
            tree = self.postTree(tree)
        commitId = self.postCommitContent({
                'subject': subject,
                'message': message,
                'tree': tree,
                'parents': parents,
                'meta': meta
            })
        sha1 = self.updateRef('branches/master', commitId, oldCommit)
        return _createEntry(
                content={'type': 'commit', 'sha1': sha1}, repo=self
            )

    def postCommitContent(self, content):
        url = '{0}/db/commits?format=minimal'.format(self.url)
        url = sign_req('POST', url)
        res = session.post(
                url, headers=contentTypeJSON, data=stringify(content),
                timeout=requestTimeout)
        _checkStatus(res, 201)
        return res.json()["data"]["_id"]

    def updateRef(self, refName, newCommit, oldCommit):
        url = '{0}/db/refs/{1}'.format(self.url, refName)
        url = sign_req('PATCH', url)
        content = {
            'new': newCommit,
            'old': oldCommit
        }
        res = session.patch(
                url, headers=contentTypeJSON, data=stringify(content),
                timeout=requestTimeout)
        _checkStatus(res, 200)
        return res.json()['data']['entry']['sha1']

    def stat(self, entries):
        url = '{0}/db/stat'.format(self.url)
        url = sign_req('POST', url)
        content = {'entries': entries}
        res = session.post(
                url, headers=contentTypeJSON, data=stringify(content),
                timeout=requestTimeout)
        _checkStatus(res, 200)
        entries = res.json()['data']['entries']
        for e in entries:
            if e['status'] == 'exists':
                self._knownEntryIds.add(e['sha1'])
        return entries

    # `postBulk()` requires that the total entry size does not violate the body
    # size limit that is enforced by the server.
    def postBulk(self, entries):
        url = '{0}/db/bulk'.format(self.url)
        url = sign_req('POST', url)
        content = {
            'entries': entries
        }
        res = session.post(
                url, headers=contentTypeJSON, data=stringify(content),
                timeout=requestTimeout)
        _checkStatus(res, 201)
        entries = res.json()['data']['entries']
        for e in entries:
            self._knownEntryIds.add(e['sha1'])
        return entries

    def linkBlob(self, sha1, path):
        self.prefetchBlob(sha1)
        self._blobCache.link(sha1, path)

    def copyBlob(self, sha1, path):
        self.prefetchBlob(sha1)
        self._blobCache.copy(sha1, path)

    def openBlob(self, sha1):
        self.prefetchBlob(sha1)
        return self._blobCache.open(sha1)

    def prefetchBlob(self, sha1):
        self._prefetchBlob(sha1)
        self._knownBlobs.add(sha1)

    # `_prefetchBlob()` may be concurrently executed in several threads.  Keep
    # it thread-safe.
    def _prefetchBlob(self, sha1):
        if self._blobCache.has(sha1):
            return
        _printmsg(0, 'Fetching blob {0}...'.format(sha1))
        self._blobCache.fetchBlob(self, sha1)
        _printmsg(0, 'Fetching blob {0} done.'.format(sha1))

    def prefetchBlobs(self, sha1s):
        with ThreadPoolExecutor(max_workers=S3_NPARALLEL) as executor:
            def prefetch(b):
                self._prefetchBlob(b)
            futures = []
            for b in set(sha1s):
                futures.append(executor.submit(prefetch, b))
            for f in as_completed(futures):
                f.result()
        for b in sha1s:
            self._knownBlobs.add(b)

    def getBlobContentUrl(self, blobId):
        url = '{0}/db/blobs/{1}/content'.format(self.url, blobId)
        return sign_req('GET', url)

    def getBlobContent(self, blobId, fp):
        url = self.getBlobContentUrl(blobId)
        chunk_size = 8 * 1024
        res = session.get(url, stream=True, timeout=requestTimeout)
        for chunk in res.iter_content(chunk_size):
            fp.write(chunk)
        _checkStatus(res, 200)

    def uploadBlobs(self, blobs):
        with ThreadPoolExecutor(max_workers=S3_NPARALLEL) as executor:
            def upload(b):
                self.uploadBlob(b)
            futures = []
            for b in blobs:
                futures.append(executor.submit(upload, b))
            for f in as_completed(futures):
                f.result()

    def uploadBlob(self, blob):
        def verifyEtag(res, data):
            etag = res.headers['etag']
            md5 = md5Buf_hex(data)
            if etag != '"{}"'.format(md5):
                msg = 'Expected ETag for MD5 {md5}; got {etag}'
                raise EtagError(msg.format(md5=md5, etag=etag))

        def putS3File(part):
            path = blob.path
            start = part["start"]
            end = part["end"]
            with open(path, 'rb') as fp:
                fp.seek(start)
                data = fp.read(end - start)
            res = session.put(part["href"], data=data, timeout=putS3Timeout)
            res.raise_for_status()
            verifyEtag(res, data)
            return res.headers["etag"]

        def putS3Buf(part):
            buf = blob.buf
            start = part["start"]
            end = part["end"]
            data = buf[start:end]
            res = session.put(part["href"], data=data, timeout=putS3Timeout)
            res.raise_for_status()
            verifyEtag(res, data)
            return res.headers["etag"]

        def getNextUploadParts(url):
            url = sign_req('GET', url)
            res = session.get(url, timeout=requestTimeout)
            _checkStatus(res, 200)
            return res.json()["data"]

        def postCompleteUpload(url, parts):
            url = sign_req('POST', url)
            content = {'s3Parts': parts}
            res = session.post(
                    url, headers=contentTypeJSON, data=json.dumps(content),
                    timeout=requestTimeout)
            _checkStatus(res, 201)
            return res.json()["data"]

        if isinstance(blob, BlobFile):
            sha1 = sha1Path_hex(blob.path)
            if sha1 != blob.sha1:
                raise RuntimeError(_denl("""
                        Sha1 mismatch for path '{0}'; expected '{1}', got
                        '{2}'.
                    """).format(blob.path, blob.sha1, sha1))
            putS3 = putS3File
            content = {'size': os.path.getsize(blob.path), 'name': blob.name}
            msg = "uploading blob {0} from path '{1}'".format(
                    blob.sha1, blob.path)
        elif isinstance(blob, BlobBuf):
            putS3 = putS3Buf
            content = {'size': len(blob.buf), 'name': blob.name}
            msg = "uploading blob {0} from anonymous buffer".format(blob.sha1)
        else:
            raise RuntimeError('Unknown blob type.')

        _printmsg(0, "{0}: {1} ... ".format(msg, content))
        res = self._startUpload(blob.sha1, content)
        if not res:
            _printmsg(0, msg + ', already available.')
            return
        completeUploadUrl = res["upload"]["href"]
        s3Parts = []
        parts = res["parts"]
        while True:
            part = parts["items"][0]
            etag = putS3(part)
            s3Parts.append({
                    'PartNumber': part["partNumber"],
                    'ETag': etag
                })
            nextPartUrl = parts["next"]
            if not nextPartUrl:
                break
            parts = getNextUploadParts(nextPartUrl)
        postCompleteUpload(completeUploadUrl, s3Parts)
        _printmsg(0, '{} ok.'.format(msg))

    def _startUpload(self, sha1, content):
        # Limit to a single part, because parts are uploaded sequentially.
        url = '{0}/db/blobs/{1}/uploads?limit=1'.format(self.url, sha1)
        url = sign_req('POST', url)
        res = session.post(
                url, headers=contentTypeJSON, data=json.dumps(content),
                timeout=requestTimeout)
        if res.status_code == 409:  # Blob already exists
            return None
        _checkStatus(res, 201)
        return res.json()["data"]

    def getCommitContent(self, sha1):
        try:
            return self._entryCache.get(sha1)
        except KeyError:
            c = self._get('db/commits/{0}?format=minimal'.format(sha1))
            idv = c['_idversion']
            if idv != 0 and idv != 1:
                msg = 'Unsupported commit idversion %d' % idv
                raise RuntimeError(msg)
            _handleErrata(c, 'commit')
            self._entryCache.add(c['_id'], c)
            self._knownEntryIds.add(sha1)
            return c

    def getTreeContent(self, sha1, recursive=False):
        if recursive:
            return self._get(
                    'db/trees/{0}?expand=99999&format=minimal'.format(sha1))
        else:
            try:
                return self._entryCache.get(sha1)
            except KeyError:
                c = self._get('db/trees/{0}?format=minimal'.format(sha1))
                idv = c['_idversion']
                if idv != 0:
                    msg = 'Unsupported tree idversion %d' % idv
                    raise RuntimeError(msg)
                _handleErrata(c, 'tree')
                self._entryCache.add(c['_id'], c)
                self._knownEntryIds.add(sha1)
                return c

    def getObjectContent(self, sha1):
        try:
            return self._entryCache.get(sha1)
        except KeyError:
            c = self._get('db/objects/{0}?format=minimal'.format(sha1))
            idv = c['_idversion']
            if idv != 0 and idv != 1:
                msg = 'Unsupported object idversion %d' % idv
                raise RuntimeError(msg)
            _handleErrata(c, 'object')
            self._entryCache.add(c['_id'], c)
            self._knownEntryIds.add(sha1)
            return c

    def _get(self, path):
        url = '{0}/{1}'.format(self.url, path)
        url = sign_req('GET', url)
        res = session.get(url, timeout=requestTimeout)
        res.encoding = 'utf-8'
        _checkStatus(res, 200)
        return res.json()["data"]

    def _getUrl(self, url):
        url = sign_req('GET', url)
        res = session.get(url, timeout=requestTimeout)
        _checkStatus(res, 200)
        return res.json()["data"]

    def _hasEntry(self, sha1):
        return sha1 in self._knownEntryIds

    def _hasBlob(self, sha1):
        return sha1 in self._knownBlobs


class PostStream:
    def __init__(self, repo):
        self.repo = repo
        self._entries = {}
        self._copyEntries = {}
        self._blobs = {}
        self._copyBlobs = {}
        self._initQueue()

    def __enter__(self):
        return self

    def __exit__(self, type, value, traceback):
        self.close()

    def _initQueue(self):
        # Don't clear entries to avoid duplicate sends after flush.  If a
        # stream has an entry that is not in `_queue`, the entry must have been
        # sent during a previous flush.
        self._queue = []
        self._bufSize = 0

    def close(self):
        self.flush()

    def flush(self):
        stat = self.repo.stat(self._queue)
        blobs = []
        entries = []
        expect = []
        for s in stat:
            sha1 = s['sha1']
            if s['status'] == 'exists':
                continue
            if s['type'] == 'blob':
                try:
                    blobs.append(self._blobs[sha1])
                except KeyError:
                    entries.append(self._copyBlobs[sha1])
                    expect.append(_pick(s, 'type', 'sha1'))
            else:
                try:
                    entries.append(self._entries[sha1])
                except KeyError:
                    entries.append(self._copyEntries[sha1])
                expect.append(_pick(s, 'type', 'sha1'))

        self.repo.uploadBlobs(blobs)

        res = self.repo.postBulk(entries)
        for e, r in zip(expect, res):
            if e != r:
                raise RuntimeError(_denl('''
                        Response entry mismatch (expected: {0}, got: {1})
                    ''').format(e, r))

        self._initQueue()

    # `_enqueue()` adds a deepcopy of `content` to ensure that the caller can
    # modify an entry after posting it.
    def _enqueue(self, type, sha1, content):
        if self._hasEntry(sha1):
            return
        self._maybeFlush(content)
        self._entries[sha1] = deepcopy(content)
        self._queue.append({'type': type, 'sha1': sha1})

    def _enqueueCopyEntry(self, type, sha1, origin):
        if self._hasEntry(sha1):
            return
        content = {
            'copy': {'type': type, 'sha1': sha1, 'repoFullName': origin}
        }
        self._maybeFlush(content)
        self._copyEntries[sha1] = content
        self._queue.append({'type': type, 'sha1': sha1})

    def _maybeFlush(self, content):
        s = len(stringify(content))
        if self._bufSize + s > POST_BUFFER_SIZE:
            self.flush()
        if s > POST_BUFFER_SIZE_LIMIT:
            raise RuntimeError(_denl('''
                   Entry too large (max JSON size is {0}; JSON for entry has
                   size {1}).
                ''').format(POST_BUFFER_SIZE_LIMIT, s))
        self._bufSize += s

    def _hasEntry(self, sha1):
        return ((sha1 in self._entries) or
                (sha1 in self._copyEntries) or
                (self.repo._hasEntry(sha1)))

    def _enqueueBlob(self, blob):
        sha1 = blob.sha1
        if self._hasBlob(sha1):
            return
        self._blobs[sha1] = blob
        self._queue.append({'type': 'blob', 'sha1': sha1})

    def _enqueueCopyBlob(self, sha1, origin):
        if self._hasBlob(sha1):
            return
        content = {
            'copy': {'type': 'blob', 'sha1': sha1, 'repoFullName': origin}
        }
        self._maybeFlush(content)
        self._copyBlobs[sha1] = content
        self._queue.append({'type': 'blob', 'sha1': sha1})

    def _hasBlob(self, sha1):
        return ((sha1 in self._blobs) or
                (sha1 in self._copyBlobs) or
                (self.repo._hasBlob(sha1)))

    def postObject(self, obj):
        return obj._postToStream(self)

    def postTree(self, tree):
        return tree._postToStream(self)


class EntryCache:
    def __init__(self):
        self._diskCache = EntryDiskCache()
        self._memCache = EntryMemCache()

    def get(self, sha1):
        try:
            return self._memCache.get(sha1)
        except KeyError:
            c = self._diskCache.get(sha1)
            self._memCache.add(sha1, c)
            return c

    def add(self, sha1, content):
        self._diskCache.add(sha1, content)
        self._memCache.add(sha1, content)


# Insert and return deepcopies to ensure that the cache is not accidentally
# modified.
class EntryMemCache:
    def __init__(self):
        self._cache = {}

    def get(self, sha1):
        return deepcopy(self._cache[sha1])

    def add(self, sha1, content):
        self._cache[sha1] = deepcopy(content)


class EntryDiskCache:
    def __init__(self, path=None):
        try:
            path = path or os.environ['NOG_CACHE_PATH']
        except KeyError:
            raise RuntimeError(_denl("""
                    Failed to get cache path from environment variable
                    `NOG_CACHE_PATH`.
                """))
        if not os.path.isdir(path):
            raise RuntimeError('Missing cache path `{0}`.'.format(path))
        path = path + '/entries'
        if not os.path.isdir(path):
            os.mkdir(path)
        self.path = path

    def get(self, sha1):
        p = self._path(sha1)
        if not os.path.exists(p):
            raise KeyError()
        with open(p, 'rb') as fp:
            jbytes = fp.read()
            content = json.loads(jbytes.decode('utf-8'))
        gotSha1 = contentId(content)
        if gotSha1 != sha1:
            raise RuntimeError(_denl("""
                    EntryDiskCache sha1 mismatch while loading entry `{0}`.
                """).format(sha1))
        return content

    def add(self, sha1, content):
        content = copy(content)
        del content['_id']
        del content['_idversion']
        gotSha1 = contentId(content)
        if gotSha1 != sha1:
            raise RuntimeError(_denl("""
                    EntryDiskCache sha1 mismatch while storing entry `{0}`: {1}
                """).format(sha1, stringify_pretty(content)))
        (dirname, filename) = self._pathPair(sha1)
        _ensureDir(dirname)
        with NamedTemporaryFile(
                mode='wb', dir=dirname, prefix='tmp-'+filename+'_',
                delete=False) as fp:
            jstring = json.dumps(content, sort_keys=True, ensure_ascii=False,
                                 separators=(',', ':'))  # canonical EJSON.
            fp.write(jstring.encode('utf-8'))
            tmpname = fp.name
        _chmodRO(tmpname)
        os.rename(tmpname, dirname + '/' + filename)

    def _path(self, sha1):
        return self.path + '/' + sha1[0:2] + '/' + sha1[2:]

    def _pathPair(self, sha1):
        return (self.path + '/' + sha1[0:2], sha1[2:])


class BlobCache:
    def __init__(self, path=None):
        try:
            path = path or os.environ['NOG_CACHE_PATH']
        except KeyError:
            raise RuntimeError(_denl("""
                    Failed to get cache path from environment variable
                    `NOG_CACHE_PATH`.
                """))
        if not os.path.isdir(path):
            raise RuntimeError('Missing cache path `{0}`.'.format(path))
        path = path + '/blobs'
        if not os.path.isdir(path):
            os.mkdir(path)
        self.path = path

    def fetchBlob(self, repo, sha1):
        (dirname, filename) = self._blobPathPair(sha1)
        _ensureDir(dirname)
        with NamedTemporaryFile(
                dir=dirname, prefix='tmp-'+filename+'_', delete=False) as fp:
            repo.getBlobContent(sha1, fp)
            tmpname = fp.name
        gotSha1 = sha1Path_hex(tmpname)
        if gotSha1 != sha1:
            os.remove(tmpname)
            raise RuntimeError(_denl("""
                    Blob sha1 mismatch (expected `{0}`, got `{1}`).
                """).format(sha1, gotSha1))
        _chmodRO(tmpname)
        os.rename(tmpname, dirname + '/' + filename)

    def has(self, sha1):
        return os.path.exists(self._blobPath(sha1))

    def link(self, sha1, dest):
        os.link(self._blobPath(sha1), dest)

    def copy(self, sha1, dest):
        shutil.copyfile(self._blobPath(sha1), dest)

    def open(self, sha1):
        return open(self._blobPath(sha1), 'rb')

    def _blobPath(self, sha1):
        return self.path + '/' + sha1[0:2] + '/' + sha1[2:]

    def _blobPathPair(self, sha1):
        return (self.path + '/' + sha1[0:2], sha1[2:])


def _chmodRO(path):
    mode = os.stat(path)[stat.ST_MODE]
    os.chmod(path, mode & ~stat.S_IWUSR & ~stat.S_IWGRP & ~stat.S_IWOTH)


def _ensureDir(dirname):
    if os.path.isdir(dirname):
        return
    try:
        os.mkdir(dirname)
    except OSError:
        if os.path.isdir(dirname):
            return
        raise


def _createEntry(content, repo):
    ty = content.get('type', None)
    if ty == 'commit':
        return Commit(content, repo)
    elif ty == 'tree':
        return Tree(content, repo)
    elif ty == 'object':
        return Object(content, repo)
    elif 'tree' in content:
        return Commit(content, repo)
    elif 'entries' in content:
        return Tree(content, repo)
    elif 'blob' in content:
        return Object(content, repo)
    else:
        raise RuntimeError('Unknown content format.')


class Entry(object):
    def __init__(self, type, content, repo):
        self._type = type
        self._repo = repo
        try:
            self._debug_sha1 = content['sha1']
            self._sha1 = content['sha1']
            self._content = None
        except KeyError:
            self._sha1 = None
            self._setContent(content)

    # Default copy() is ok.

    # Implement deepcopy to copy only `_content`, but not `_repo`.
    def __deepcopy__(self, memo):
        if self._sha1:
            # No nested data structures without content; simply copy.
            return copy(self)
        return _createEntry(deepcopy(self._content, memo), self._repo)

    @property
    def sha1(self):
        if self._sha1:
            return self._sha1
        return contentId(self.content)

    @property
    def content(self):
        self._ensureContent()
        return self._content

    @property
    def type(self):
        return self._type

    @property
    def name(self):
        self._ensureContent()
        return self._content['name']

    @name.setter
    def name(self, n):
        self._ensureContent()
        self._content['name'] = n

    @property
    def meta(self):
        self._ensureContent()
        return self._content['meta']

    def _ensureContent(self):
        if self._content:
            pass
        elif self._type == 'commit':
            self._setContent(self._repo.getCommitContent(self._sha1))
        elif self._type == 'tree':
            self._setContent(self._repo.getTreeContent(self._sha1))
        elif self._type == 'object':
            self._setContent(self._repo.getObjectContent(self._sha1))
        else:
            raise RuntimeError('Unknown entry type.')
        self._sha1 = None

    def _setContent(self, content):
        for k in ('_id', '_idversion'):
            try:
                del content[k]
            except KeyError:
                pass
        self._sha1 = None
        self._content = content


# ISO datetime with UTC `Z`.
_rgxISOStringUTC = re.compile(
    r"""
        ^
        [0-9]{4}-[0-9]{2}-[0-9]{2}
        T
        [0-9]{2}:[0-9]{2}:[0-9]{2}
        Z
        $
    """, re.X)


class Commit(Entry):
    def __init__(self, content, repo):
        if sys.version_info.major == 3:
            super().__init__('commit', content, repo)
        else:
            Entry.__init__(self, 'commit', content, repo)

    @property
    def idversion(self):
        self._ensureContent()
        if _rgxISOStringUTC.match(self._content['authorDate']):
            return 0
        return 1

    @property
    def tree(self):
        self._ensureContent()
        return _createEntry(
                content={'type': 'tree', 'sha1': self._content['tree']},
                repo=self._repo
            )

    @property
    def parents(self):
        self._ensureContent()
        ps = []
        for p in self._content['parents']:
            ps.append(_createEntry(content={'type': 'commit', 'sha1': p},
                                   repo=self._repo))
        return ps

    @property
    def subject(self):
        self._ensureContent()
        return self._content['subject']

    @property
    def message(self):
        self._ensureContent()
        return self._content['message']

    @property
    def authors(self):
        self._ensureContent()
        return self._content['authors']

    @property
    def authorDate(self):
        self._ensureContent()
        return _asPythonDate(self._content['authorDate'])

    @property
    def committer(self):
        self._ensureContent()
        return self._content['committer']

    @property
    def commitDate(self):
        self._ensureContent()
        return _asPythonDate(self._content['commitDate'])


# Use `dateutil` to parse the string, since it immediately provides
# compatibilty with Python 2.7.
def _asPythonDate(datestr):
    return dateparse(datestr)


def _isFromOtherRepo(stream, entry):
    return (entry._repo and (entry._repo is not stream.repo))


class Tree(Entry):
    def __init__(self, content=None, repo=None):
        content = content or {'name': '', 'entries': [], 'meta': {}}
        if sys.version_info.major == 3:
            super().__init__('tree', content, repo)
        else:
            Entry.__init__(self, 'tree', content, repo)

    @property
    def content(self):
        self._ensureContent()
        content = copy(self._content)
        collapsed = []
        for e in content['entries']:
            if isinstance(e, Entry):
                collapsed.append({'type': e.type, 'sha1': e.sha1})
            else:
                collapsed.append(e)
        content['entries'] = collapsed
        return content

    def collapse(self):
        collapsed = []
        for e in self._content['entries']:
            if isinstance(e, Object):
                collapsed.append({'type': 'object', 'sha1': e.sha1})
            elif isinstance(e, Tree):
                e.collapse()
                collapsed.append({'type': 'tree', 'sha1': e.sha1})
            elif isinstance(e, Entry):
                raise RuntimeError('Invalid entry type.')
            else:
                collapsed.append(e)
        self._content['entries'] = collapsed

    def _postToStream(self, stream):
        def walk(tree):
            if tree._sha1:
                if _isFromOtherRepo(stream, tree):
                    stream._enqueueCopyEntry('tree', tree._sha1,
                                             tree._repo.fullName)
                # Do not set `tree._repo` to the stream's repo to support
                # retries.  If `PostStream.flush()` fails, retry needs the
                # original repo.
                return tree._sha1
            content = copy(tree._content)
            collapsed = []
            for e in content['entries']:
                if isinstance(e, Object):
                    collapsed.append({
                            'type': 'object', 'sha1': stream.postObject(e)
                        })
                elif isinstance(e, Tree):
                    collapsed.append({'type': 'tree', 'sha1': walk(e)})
                elif isinstance(e, Entry):
                    raise RuntimeError('Invalid entry type.')
                else:
                    if _isFromOtherRepo(stream, tree):
                        stream._enqueueCopyEntry(e['type'], e['sha1'],
                                                 tree._repo.fullName)
                    collapsed.append(e)
            content['entries'] = collapsed
            sha1 = contentId(content)
            stream._enqueue('tree', sha1, content)
            tree._repo = stream.repo
            return sha1
        return walk(self)

    def append(self, e):
        self._ensureContent()
        self._content['entries'].append(e)

    def insert(self, i, e):
        self._ensureContent()
        self._content['entries'].insert(i, e)

    def pop(self, *args):
        self._ensureContent()
        return self._content['entries'].pop(*args)

    def entries(self, pattern=None, type=None):
        for idx, e in self.enumerateEntries(pattern, type):
            yield e

    def enumerateEntries(self, pattern=None, type=None):
        self._ensureContent()
        entries = self._content['entries']
        for idx, e in enumerate(entries):
            if isinstance(e, Entry):
                if type and e.type != type:
                    continue
            else:
                if type and (e['type'] != type):
                    continue
                e = _createEntry(content=e, repo=self._repo)
                entries[idx] = e
            if not pattern or fnmatch(e.name, pattern):
                yield (idx, e)

    def objects(self, pattern=None):
        for idx, e in self.enumerateObjects(pattern):
            yield e

    def enumerateObjects(self, pattern=None):
        return self.enumerateEntries(pattern, type='object')

    def trees(self, pattern=None):
        for idx, e in self.enumerateTrees(pattern):
            yield e

    def enumerateTrees(self, pattern=None):
        return self.enumerateEntries(pattern, type='tree')


_rgxSha1 = re.compile(r'^[0-9a-f]{40}$')


def isSha1(sha1):
    return (isinstance(sha1, str) and _rgxSha1.match(sha1))


class Object(Entry):
    def __init__(self, content=None, repo=None):
        # Default idversion is 1.
        content = content or {'name': '', 'meta': {},
                              'blob': None, 'text': None}
        if sys.version_info.major == 3:
            super().__init__('object', content, repo)
        else:
            Entry.__init__(self, 'object', content, repo)

    @property
    def idversion(self):
        self._ensureContent()
        if 'text' in self._content:
            return 1
        return 0

    def format(self, idversion):
        if self.idversion == idversion:
            return
        if idversion == 0:
            content = deepcopy(self._content)
            content['meta']['content'] = content['text']
            del content['text']
            if not content['blob']:
                content['blob'] = NULL_SHA1
            self._setContent(content)
        elif idversion == 1:
            content = deepcopy(self._content)
            if 'content' in content['meta']:
                content['text'] = content['meta']['content']
                del content['meta']['content']
            else:
                content['text'] = None
            if content['blob'] == NULL_SHA1:
                content['blob'] = None
            self._setContent(content)
        else:
            raise RuntimeError('Invalid idversion.')

    @property
    def content(self):
        self._ensureContent()
        content = self._content
        if content['blob'] is None:
            return content
        if isinstance(content['blob'], six.text_type):
            return content
        content = copy(content)
        content['blob'] = content['blob'].sha1
        return content

    def _postToStream(self, stream):
        if self._sha1:
            if _isFromOtherRepo(stream, self):
                stream._enqueueCopyEntry('object', self._sha1,
                                         self._repo.fullName)
            # Do not set `self._repo` to the stream's repo to support retries.
            # If `PostStream.flush()` fails, the retry needs the original repo.
            return self._sha1

        content = self._content
        if content['blob'] is None or content['blob'] == NULL_SHA1:
            pass
        elif isinstance(content['blob'], str):
            if _isFromOtherRepo(stream, self):
                stream._enqueueCopyBlob(content['blob'], self._repo.fullName)
        else:
            stream._enqueueBlob(content['blob'])
            content = copy(content)
            content['blob'] = content['blob'].sha1

        if self.idversion == 1 and 'content' in self.meta:
            msg = (
                    'Invalid Object `{}` in idversion 1: '
                    "use `obj.text` instead of `obj.meta['content']`."
            )
            raise RuntimeError(msg.format(self.name))

        sha1 = contentId(content)
        content = deepcopy(content)
        content['_idversion'] = self.idversion
        stream._enqueue('object', sha1, content)
        self._repo = stream.repo
        return sha1

    @property
    def text(self):
        self._ensureContent()
        if self.idversion == 0:
            return self._content['meta'].get('content', None)
        else:
            return self._content['text']

    @text.setter
    def text(self, text):
        self._ensureContent()
        if self.idversion == 0:
            self._content['meta']['content'] = text
        else:
            self._content['text'] = text

    # XXX: We should switch, probably soon, to returning NULL_SHA1 as None.
    @property
    def blob(self):
        self._ensureContent()
        b = self._content['blob']
        try:
            return b.sha1
        except AttributeError:
            return b

    @blob.setter
    def blob(self, blob):
        self._ensureContent()
        if blob is None or blob == NULL_SHA1:
            if self.idversion == 0:
                self._content['blob'] = NULL_SHA1
            else:
                self._content['blob'] = None
        elif isinstance(blob, str) and _rgxSha1.match(blob):
            self._content['blob'] = blob
        elif isinstance(blob, str) and os.path.exists(blob):
            self._content['blob'] = BlobFile(blob)
        elif isinstance(blob, bytes):
            self._content['blob'] = BlobBuf(blob)
        else:
            raise RuntimeError('Unknown blob type.')

    def linkBlob(self, path):
        sha1 = self.blob
        self._repo.prefetchBlob(sha1)
        self._repo._blobCache.link(sha1, path)

    def copyBlob(self, path):
        sha1 = self.blob
        self._repo.prefetchBlob(sha1)
        self._repo._blobCache.copy(sha1, path)

    def openBlob(self):
        sha1 = self.blob
        self._repo.prefetchBlob(sha1)
        return self._repo._blobCache.open(sha1)


class BlobBuf:
    def __init__(self, buf):
        self.sha1 = sha1Buf_hex(buf)
        self.buf = buf
        self.name = 'anonymous buffer'


class BlobFile:
    def __init__(self, path):
        self.sha1 = sha1Path_hex(path)
        self.path = path
        self.name = os.path.basename(path)


def postJobStatus(jobId, retryId, status, reason=None):
    url = '{0}/jobs/{1}/status'.format(_apiUrl(), jobId)
    url = sign_req('POST', url)
    content = {
        'retryId': retryId,
        'status': status
    }
    if reason:
        content['reason'] = reason
    res = session.post(url, headers=contentTypeJSON, data=stringify(content),
                       timeout=requestTimeout)
    _checkStatus(res, 200)


def postJobProgress(jobId, retryId, completed, total):
    url = '{0}/jobs/{1}/progress'.format(_apiUrl(), jobId)
    url = sign_req('POST', url)
    content = {
        'retryId': retryId,
        'progress': {'completed': completed, 'total': total}
    }
    res = session.post(url, headers=contentTypeJSON, data=stringify(content),
                       timeout=requestTimeout)
    _checkStatus(res, 200)


def postJobLog(jobId, retryId, message, level=None):
    url = '{0}/jobs/{1}/log'.format(_apiUrl(), jobId)
    url = sign_req('POST', url)
    content = {
        'retryId': retryId,
        'message': message
    }
    if level:
        content['level'] = level
    res = session.post(url, headers=contentTypeJSON, data=stringify(content),
                       timeout=requestTimeout)
    _checkStatus(res, 200)


def sign_req(method, url):
    authkeyid = os.environ['NOG_KEYID']
    secretkey = os.environ['NOG_SECRETKEY'].encode()
    authalgorithm = 'nog-v1'
    authdate = datetime.utcnow().strftime('%Y-%m-%dT%H%M%SZ')
    authexpires = '600'
    authnonce = hexlify(os.urandom(5)).decode('utf-8')

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


def _checkStatus(res, expectedStatusCode):
    if res.status_code == expectedStatusCode:
        return
    msg = 'Unexpected status code; expected {0}, got {1}.'
    msg = msg.format(expectedStatusCode, res.status_code)
    msg = msg + '  URL: ' + res.request.url
    try:
        data = res.json()
        msg = msg + '\nResponse JSON: ' + stringify_pretty(data)
    except:
        pass
    raise RuntimeError(msg)


# `denl()` converts a multiline string to a single line, joining lines with
# single spaces.
def _denl(s):
    return re.sub('\s*\n\s*', ' ', s)


def stringify_pretty(d):
    return json.dumps(d, sort_keys=True, ensure_ascii=False, indent=2) + '\n'


def stringify_canonical(d):
    return json.dumps(
            d, sort_keys=True, ensure_ascii=False, separators=(',', ':')
        ).encode('utf-8')


def stringify(d):
    return json.dumps(
            d, ensure_ascii=False, separators=(',', ':')
        ).encode('utf-8')


def contentId(e):
    h = hashlib.sha1()
    h.update(stringify_canonical(e))
    return h.hexdigest()


def _pick(d, *args):
    return {k: v for k, v in d.items() if k in args}


def sha1Path_hex(path):
    BLOCKSIZE = 8 * 1024
    h = hashlib.sha1()
    with open(path, 'rb') as fp:
        while True:
            buf = fp.read(BLOCKSIZE)
            if len(buf) == 0:
                break
            h.update(buf)
    return h.hexdigest()


def sha1Buf_hex(buf):
    h = hashlib.sha1()
    h.update(buf)
    return h.hexdigest()


def md5Buf_hex(buf):
    h = hashlib.md5()
    h.update(buf)
    return h.hexdigest()
