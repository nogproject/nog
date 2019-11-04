#!/usr/bin/env python3

"""\
nog lib

See `main()` for usage example.
"""

from binascii import hexlify
from concurrent.futures import ThreadPoolExecutor, as_completed
from copy import copy, deepcopy
from datetime import datetime
from fnmatch import fnmatch
from random import getrandbits
from requests.adapters import HTTPAdapter
from tempfile import NamedTemporaryFile
from textwrap import dedent
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
if sys.version_info.major == 3:
    from urllib.parse import urlparse
else:
    from urlparse import urlparse

NULL_SHA1 = '0000000000000000000000000000000000000000'
contentTypeJSON = {'Content-Type': 'application/json'}

# Number of parallel uploads to S3.
S3_NPARALLEL = 32

verbosity = 0

session = requests.Session()
session.mount(
        os.environ.get('NOG_API_URL', 'http://localhost:3000/api'),
        HTTPAdapter(max_retries=os.environ.get('NOG_MAX_RETRIES', 0))
    )


def _printmsg(level, msg):
    if level <= verbosity:
        print(msg)


def main():
    testdir = 'nog-py-testdir'
    if not os.path.isdir(testdir):
        os.mkdir(testdir)
    os.chdir(testdir)

    if not os.path.isdir('cache'):
        os.mkdir('cache')
    os.environ['NOG_CACHE_PATH'] = os.getcwd() + '/cache'

    testrepo = os.environ.get(
            'NOG_TEST_REPO', 'nog-py-testrepo-%x' % getrandbits(2))
    try:
        remote = openRepo(testrepo)
        master = remote.getMaster()
        # Reset master as if it was a new repo.
        remote.updateRef('branches/master', NULL_SHA1, master.sha1)
    except RuntimeError:
        remote = createRepo(testrepo)
    print('repo url:', remote.url)
    master = remote.getMaster()

    root = Tree()
    root.meta['study'] = 'nog-py-test'
    data = Tree()
    data.name = 'datalist'
    root.append(data)
    results = Tree()
    results.name = 'results'
    root.append(results)
    master = remote.commitTree(
            subject='Create initial workspace layout', tree=root, parents=[],
            oldCommit=NULL_SHA1
        )
    print(stringify_pretty(master.content))

    for i in range(5):
        obj = Object()
        randstr = ('%x' % getrandbits(8))
        obj.name = 'random-' + randstr
        obj.meta['specimen'] = randstr
        obj.meta['stage'] = 'input'
        obj.blob = randstr.encode('utf-8')
        data.append(obj)

    randstr = ('%x' % getrandbits(8))
    path = 'random-' + randstr + '-from-file'
    with open(path, 'w') as fp:
        fp.write(randstr)
    obj = Object()
    obj.name = path
    obj.meta['specimen'] = randstr
    obj.meta['stage'] = 'input'
    obj.blob = './' + path  # Keep path until tree has been committed.
    data.append(obj)

    master = remote.commitTree(
            subject='Add data', tree=root, parents=[master.sha1]
        )
    print(stringify_pretty(master.content))
    os.remove(path)  # Now it's ok to remove it.

    # Create a summary report and one result tree for each specimen.  Each
    # result tree contains a specimen-specific, report, the input as a subtree,
    # and futher result data.
    summary = Object()
    results.append(summary)
    summary.name = 'summary.md'
    sumd = '# Summary Report\n\n'

    remote.prefetchBlobs(d.blob for d in data.objects())
    for d in data.objects():
        name = d.name
        specimen = d.meta['specimen']
        sumd = sumd + (' - [specimen {0}]({0}): {1}\n'.format(specimen, name))

        restree = Tree()
        restree.name = specimen
        restree.meta['specimen'] = specimen
        restree.meta['kind'] = 'SpecimenResultTree'

        report = Object()
        report.name = 'report.md'
        report.meta['content'] = '# Report {0}\n\n'.format(specimen)
        restree.append(report)

        input = Tree()
        input.name = 'input'
        input.meta['specimen'] = specimen
        input.append(d)
        restree.append(input)

        # Link the blob to a file to illustrate how analysis code that expects
        # files could be executed.
        d.linkBlob(name)
        with open(name) as fp:
            dat = fp.read()
        os.remove(name)
        res = Object()
        res.meta['specimen'] = specimen
        res.meta['stage'] = 'link analysis'
        res.name = 'result-' + name + '-ln'
        res.meta['content'] = dat
        restree.append(res)

        # Open blob directly as fp to illustrate how code that can read from a
        # stream could be executed.
        with d.openBlob() as fp:
            data = fp.read()
        res = Object()
        res.meta['specimen'] = specimen
        res.meta['stage'] = 'open analysis'
        res.name = 'result-' + name + '-open'
        res.meta['content'] = dat
        restree.append(res)

        results.append(restree)

    summary.meta['content'] = sumd

    master = remote.commitTree(
            subject='Add results', tree=root, parents=[master.sha1]
        )
    print(stringify_pretty(master.content))

    print('repo url:', remote.url)


def openRepo(name):
    name = _completeRepoName(name)
    apiUrl = os.environ.get('NOG_API_URL', 'http://localhost:3000/api')
    remote = RemoteRepo(apiUrl + '/repos/' + name)
    remote.getMaster()
    return remote


def createRepo(name):
    name = _completeRepoName(name)
    apiUrl = os.environ.get('NOG_API_URL', 'http://localhost:3000/api')
    url = apiUrl + '/repos'
    url = sign_req('POST', url)
    content = {
        'repoFullName': name
    }
    res = session.post(
            url, headers=contentTypeJSON, data=stringify(content))
    _checkStatus(res, 201)
    return RemoteRepo(apiUrl + '/repos/' + name)


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

    def commitTree(
            self, subject, tree, parents, oldCommit=None, message=None,
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
                url, headers=contentTypeJSON, data=stringify(content))
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
                url, headers=contentTypeJSON, data=stringify(content))
        _checkStatus(res, 200)
        return res.json()['data']['entry']['sha1']

    def stat(self, entries):
        url = '{0}/db/stat'.format(self.url)
        url = sign_req('POST', url)
        content = {'entries': entries}
        res = session.post(
                url, headers=contentTypeJSON, data=stringify(content))
        _checkStatus(res, 200)
        entries = res.json()['data']['entries']
        for e in entries:
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
                url, headers=contentTypeJSON, data=stringify(content))
        _checkStatus(res, 201)
        entries = res.json()['data']['entries']
        for e in entries:
            self._knownEntryIds.add(e['sha1'])
        return entries

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
            for b in set(sha1s):
                executor.submit(prefetch, b)
        for b in sha1s:
            self._knownBlobs.add(b)

    def getBlobContent(self, blobId, fp):
        url = '{0}/db/blobs/{1}/content'.format(self.url, blobId)
        url = sign_req('GET', url)
        chunk_size = 8 * 1024
        res = session.get(url, stream=True)
        for chunk in res.iter_content(chunk_size):
            fp.write(chunk)
        _checkStatus(res, 200)

    def uploadBlob(self, blob):
        def putS3File(part):
            path = blob.path
            start = part["start"]
            end = part["end"]
            with open(path, 'rb') as fp:
                fp.seek(start)
                data = fp.read(end - start)
            res = session.put(part["href"], data=data)
            res.raise_for_status()
            return res.headers["etag"]

        def putS3Buf(part):
            buf = blob.buf
            start = part["start"]
            end = part["end"]
            res = session.put(part["href"], data=buf[start:end])
            res.raise_for_status()
            return res.headers["etag"]

        def getNextUploadParts(url):
            url = sign_req('GET', url)
            res = session.get(url)
            _checkStatus(res, 200)
            return res.json()["data"]

        def postCompleteUpload(url, parts):
            url = sign_req('POST', url)
            content = {'s3Parts': parts}
            res = session.post(
                    url, headers=contentTypeJSON, data=json.dumps(content))
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
        _printmsg(0, msg + 'ok.')

    def _startUpload(self, sha1, content):
        # Limit to a single part, because parts are uploaded sequentially.
        url = '{0}/db/blobs/{1}/uploads?limit=1'.format(self.url, sha1)
        url = sign_req('POST', url)
        res = session.post(
                url, headers=contentTypeJSON, data=json.dumps(content))
        if res.status_code == 409:  # Blob already exists
            return None
        _checkStatus(res, 201)
        return res.json()["data"]

    def getCommitContent(self, sha1):
        try:
            return self._entryCache.get(sha1)
        except KeyError:
            c = self._get('db/commits/{0}?format=minimal'.format(sha1))
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
                self._entryCache.add(c['_id'], c)
                self._knownEntryIds.add(sha1)
                return c

    def getObjectContent(self, sha1):
        try:
            return self._entryCache.get(sha1)
        except KeyError:
            c = self._get('db/objects/{0}?format=minimal'.format(sha1))
            self._entryCache.add(c['_id'], c)
            self._knownEntryIds.add(sha1)
            return c

    def _get(self, path):
        url = '{0}/{1}'.format(self.url, path)
        url = sign_req('GET', url)
        res = session.get(url)
        _checkStatus(res, 200)
        return res.json()["data"]

    def _getUrl(self, url):
        url = sign_req('GET', url)
        res = session.get(url)
        _checkStatus(res, 200)
        return res.json()["data"]

    def _hasEntry(self, sha1):
        return sha1 in self._knownEntryIds

    def _hasBlob(self, sha1):
        return sha1 in self._knownBlobs


class PostStream:
    def __init__(self, repo):
        self.repo = repo
        self.BUF_SIZE = 5000000
        # self.BUF_SIZE = 50000
        # self.BUF_SIZE = 500
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

        with ThreadPoolExecutor(max_workers=S3_NPARALLEL) as executor:
            def upload(b):
                self.repo.uploadBlob(b)
            futures = []
            for b in blobs:
                futures.append(executor.submit(upload, b))
            for f in as_completed(futures):
                f.result()

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
        l = len(stringify(content))
        if self._bufSize + l > self.BUF_SIZE:
            self.flush()
        if self._bufSize + l > self.BUF_SIZE:
            raise RuntimeError(_denl('''
                   Entry too large (max JSON size is {0}; JSON for entry has
                   size {1}).
                ''').format(self.BUF_SIZE, l))
        self._bufSize = self._bufSize + l

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
        with open(p, 'r') as fp:
            content = json.load(fp)
        gotSha1 = contentId(content)
        if gotSha1 != sha1:
            raise RuntimeError(_denl("""
                    EntryDiskCache sha1 mismatch while loading entry `{0}`.
                """).format(sha1))
        return content

    def add(self, sha1, content):
        content = copy(content)
        del content['_id']
        gotSha1 = contentId(content)
        if gotSha1 != sha1:
            raise RuntimeError(_denl("""
                    EntryDiskCache sha1 mismatch while storing entry `{0}`: {1}
                """).format(sha1, stringify_pretty(content)))
        (dirname, filename) = self._pathPair(sha1)
        _ensureDir(dirname)
        with NamedTemporaryFile(
                mode='w', dir=dirname, prefix='tmp-'+filename+'_',
                delete=False) as fp:
            json.dump(content, fp, sort_keys=True, ensure_ascii=False,
                      separators=(',', ':'))  # canonical EJSON.
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
            return
        if self._type == 'commit':
            self._setContent(self._repo.getCommitContent(self._sha1))
        elif self._type == 'tree':
            self._setContent(self._repo.getTreeContent(self._sha1))
        elif self._type == 'object':
            self._setContent(self._repo.getObjectContent(self._sha1))
        else:
            raise RuntimeError('Unknown entry type.')

    def _setContent(self, content):
        try:
            del content['_id']
        except KeyError:
            pass
        self._sha1 = None
        self._content = content


class Commit(Entry):
    def __init__(self, content, repo):
        if sys.version_info.major == 3:
            super().__init__('commit', content, repo)
        else:
            Entry.__init__(self,'commit', content, repo)

    @property
    def tree(self):
        self._ensureContent()
        return _createEntry(
                content={'type': 'tree', 'sha1': self._content['tree']},
                repo=self._repo
            )


def _isFromOtherRepo(stream, entry):
    return (entry._repo and (entry._repo is not stream.repo))


class Tree(Entry):
    def __init__(self, content=None, repo=None):
        content = content or {'name': '', 'entries': [], 'meta': {}}
        if sys.version_info.major == 3:
            super().__init__('tree', content, repo)
        else:
            Entry.__init__(self,'tree', content, repo)

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

    def _postToStream(self, stream):
        def walk(tree):
            if tree._sha1:
                if _isFromOtherRepo(stream, tree):
                    stream._enqueueCopyEntry('tree', tree._sha1,
                                             tree._repo.fullName)
                tree._repo = stream.repo
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
        content = content or {'name': '', 'meta': {}, 'blob': NULL_SHA1}
        if sys.version_info.major == 3:
            super().__init__('object', content, repo)
        else:
            Entry.__init__(self,'object', content, repo)

    @property
    def content(self):
        self._ensureContent()
        content = self._content
        if isinstance(content['blob'], str):
            return content
        content = copy(content)
        content['blob'] = content['blob'].sha1
        return content

    def _postToStream(self, stream):
        if self._sha1:
            if _isFromOtherRepo(stream, self):
                stream._enqueueCopyEntry('object', self._sha1,
                                         self._repo.fullName)
            self._repo = stream.repo
            return self._sha1
        content = self._content
        if isinstance(content['blob'], str):
            if _isFromOtherRepo(stream, self):
                stream._enqueueCopyBlob(content['blob'], self._repo.fullName)
        else:
            stream._enqueueBlob(content['blob'])
            content = copy(content)
            content['blob'] = content['blob'].sha1
        sha1 = contentId(content)
        stream._enqueue('object', sha1, content)
        self._repo = stream.repo
        return sha1

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
        if not blob:
            self._content['blob'] = NULL_SHA1
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
    apiUrl = os.environ.get('NOG_API_URL', 'http://localhost:3000/api')
    url = '{0}/jobs/{1}/status'.format(apiUrl, jobId)
    url = sign_req('POST', url)
    content = {
        'retryId': retryId,
        'status': status
    }
    if reason:
        content['reason'] = reason
    res = session.post(url, headers=contentTypeJSON, data=stringify(content))
    _checkStatus(res, 200)


def postJobProgress(jobId, retryId, completed, total):
    apiUrl = os.environ.get('NOG_API_URL', 'http://localhost:3000/api')
    url = '{0}/jobs/{1}/progress'.format(apiUrl, jobId)
    url = sign_req('POST', url)
    content = {
        'retryId': retryId,
        'progress': {'completed': completed, 'total': total}
    }
    res = session.post(url, headers=contentTypeJSON, data=stringify(content))
    _checkStatus(res, 200)


def postJobLog(jobId, retryId, message, level=None):
    apiUrl = os.environ.get('NOG_API_URL', 'http://localhost:3000/api')
    url = '{0}/jobs/{1}/log'.format(apiUrl, jobId)
    url = sign_req('POST', url)
    content = {
        'retryId': retryId,
        'message': message
    }
    if level:
        content['level'] = level
    res = session.post(url, headers=contentTypeJSON, data=stringify(content))
    _checkStatus(res, 200)


def sign_req(method, url):
    authkeyid = os.environ['NOG_KEYID']
    secretkey = os.environ['NOG_SECRETKEY'].encode()
    authalgorithm = 'nog-v1'
    authdate = datetime.utcnow().strftime('%Y-%m-%dT%H%M%SZ')
    authexpires = '600'
    authnonce = ('%x' % getrandbits(40))

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


if __name__ == '__main__':
    main()
