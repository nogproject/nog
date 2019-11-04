"""

Run tests with:

    NOG_CACHE_PATH=$(mktemp -d -t testing_nogpy-XXXXXXX) py.test

"""


from copy import deepcopy
import hashlib
import nog
import os
import sys
import pytest
if sys.version_info.major == 3:
    from datetime import datetime, timezone, timedelta
else:
    from datetime import datetime, timedelta
    from pytz import timezone
    import pytz


def sha1(b):
    h = hashlib.sha1()
    h.update(b)
    return h.hexdigest()


repoName = 'testing_nogpy'
remote = None

NULL_SHA1 = u'0000000000000000000000000000000000000000'

objContent_v0 = {
    'blob': NULL_SHA1,
    'meta': {'content': 'text'},
    'name': 'foo'
}
objId_v0 = 'e306bba8afcead972947bba6627d7f3e3cfeef51'

objContent_v1 = {
    'blob': None,
    'meta': {},
    'name': 'foo',
    'text': 'text'
}
objId_v1 = 'a5c7dadaae838f765f66d3d354617a6e564fdc59'

blobBytes = 'file content\n'.encode('utf-8')
blobSha1 = sha1(blobBytes)
emptyBlobSha1 = 'da39a3ee5e6b4b0d3255bfef95601890afd80709'

treeContent = {
    'meta': {'foo': 'bar'},
    'name': 'tree',
    'entries': [
        {'type': 'object', 'sha1': objId_v0},
        {'type': 'object', 'sha1': objId_v1},
    ]
}
treeId = '909841620c9e56a9b874042ca44a5694b6622e8b'

# We use low-level APIs to construct datetimes here in order to test the
# integration of `parse()` in the implementation.
isoUTCZ = '2015-11-01T00:00:00Z'
isoTZPlus = '2015-11-01T00:00:00+01:00'
isoTZMinus = '2015-11-01T00:00:00-06:00'
if sys.version_info.major == 3:
    dateUTCZ = datetime(2015, 11, 1, tzinfo=timezone.utc)
    tzPlus = timezone(timedelta(hours=1, minutes=0))
    dateTZPlus = datetime(2015, 11, 1, tzinfo=tzPlus)
    tzMinus = timezone(timedelta(hours=-6, minutes=0))
    dateTZMinus = datetime(2015, 11, 1, tzinfo=tzMinus)
else:
    tzPlus = timezone('Etc/GMT-1')
    tzMinus = timezone('Etc/GMT+6')
    dateUTCZ = datetime(2015, 11, 1, tzinfo=pytz.utc)
    dateTZPlus = tzPlus.localize(datetime(2015, 11, 1))
    dateTZMinus = tzMinus.localize(datetime(2015, 11, 1))
AUThor = 'A. U. Thor <author@example.com>'


# Check that the expected variables are set before running the actual tests.
def test_env():
    vars = ('NOG_API_URL', 'NOG_CACHE_PATH', 'NOG_USERNAME', 'NOG_KEYID',
            'NOG_SECRETKEY')
    for v in vars:
        assert(v in os.environ)


# Do not create a new repo for each test run, because it is annoying to clean
# up.  Only ensure that the testing repo is present.
def test_ensureTestingRepo():
    global remote
    try:
        remote = nog.openRepo(repoName)
    except RuntimeError:
        remote = nog.createRepo(repoName)
    assert remote.url.endswith(repoName)


def test_default_Object_idversion():
    obj = nog.Object()
    assert obj.idversion == 1


def test_fundamental_Object_operations():
    obj_v1 = nog.Object()
    obj_v1.name = 'foo'
    obj_v1.text = 'text'
    assert obj_v1.content == objContent_v1
    assert obj_v1.sha1 == objId_v1

    obj_v0 = deepcopy(obj_v1)
    obj_v0.format(0)
    assert obj_v0.content == objContent_v0
    assert obj_v0.sha1 == objId_v0

    assert remote.postObject(obj_v0) == objId_v0
    assert remote.postObject(obj_v1) == objId_v1

    obj = remote.getObject(objId_v0)
    assert obj.idversion == 0
    assert obj.sha1 == objId_v0
    assert obj.content == objContent_v0
    assert obj.text == 'text'
    assert obj.meta == {'content': 'text'}
    assert obj.blob == NULL_SHA1

    obj = remote.getObject(objId_v1)
    assert obj.idversion == 1
    assert obj.sha1 == objId_v1
    assert obj.content == objContent_v1
    assert obj.text == 'text'
    assert obj.meta == {}
    assert obj.blob is None

    obj.format(0)
    assert obj.sha1 == objId_v0

    assert nog.Object(content=objContent_v0).sha1 == objId_v0
    assert nog.Object(content=objContent_v1).sha1 == objId_v1


def test_reject_invalid_idv1_obj():
    obj = nog.Object()
    obj.name = 'foo'
    obj.meta['content'] = 'bar'
    with pytest.raises(RuntimeError) as errinfo:
        remote.postObject(obj)
    assert('Invalid Object' in str(errinfo.value))
    assert('foo' in str(errinfo.value))
    assert('idversion 1' in str(errinfo.value))


def test_no_false_sha1_caching():
    obj = remote.getObject(objId_v1)
    assert obj.sha1 == objId_v1
    obj.name = 'new name'
    assert obj.sha1 != objId_v1

    obj = remote.getObject(objId_v1)
    assert obj.sha1 == objId_v1
    obj.blob = blobBytes
    assert obj.sha1 != objId_v1

    obj = remote.getObject(objId_v1)
    assert obj.sha1 == objId_v1
    obj.meta['new'] = 'field'
    assert obj.sha1 != objId_v1

    obj = remote.getObject(objId_v1)
    assert obj.sha1 == objId_v1
    obj.text = 'new text'
    assert obj.sha1 != objId_v1


def test_blob_from_file(tmpdir):
    inpath = tmpdir.join('a.dat')
    inpath.write(blobBytes)

    obj = nog.Object()
    obj.blob = str(inpath)
    assert obj.blob == blobSha1

    sha1 = remote.postObject(obj)
    obj = remote.getObject(sha1)
    assert obj.blob == blobSha1
    with obj.openBlob() as fp:
        assert fp.read() == blobBytes

    cppath = tmpdir.join('cp.dat')
    obj.copyBlob(str(cppath))
    with cppath.open('rb') as fp:
        assert fp.read() == blobBytes

    lnpath = tmpdir.join('ln.dat')
    obj.linkBlob(str(lnpath))
    with lnpath.open('rb') as fp:
        assert fp.read() == blobBytes


def test_blob_from_bytes():
    obj = nog.Object()
    obj.blob = blobBytes
    assert obj.blob == blobSha1

    sha1 = remote.postObject(obj)
    obj = remote.getObject(sha1)
    assert obj.blob == blobSha1


def test_blob_from_sha1():
    obj = nog.Object()
    obj.blob = blobSha1
    assert obj.blob == blobSha1

    sha1 = remote.postObject(obj)
    obj = remote.getObject(sha1)
    assert obj.blob == blobSha1


def test_empty_blob():
    obj = nog.Object()
    obj.blob = b''
    assert obj.blob == emptyBlobSha1

    sha1 = remote.postObject(obj)
    obj = remote.getObject(sha1)
    assert obj.blob == emptyBlobSha1


def test_blob_random_bytes_to_force_upload():
    obj = nog.Object()
    buf = os.urandom(20)
    bufSha1 = sha1(buf)
    obj.blob = buf
    assert obj.blob == bufSha1

    oid = remote.postObject(obj)
    obj = remote.getObject(oid)
    assert obj.blob == bufSha1


def test_blob_setter():
    obj = nog.Object()
    assert obj.idversion == 1
    obj.blob = None
    assert obj.blob is None
    obj.blob = NULL_SHA1
    assert obj.blob is None

    obj.format(0)
    assert obj.idversion == 0
    obj.blob = None
    assert obj.blob == NULL_SHA1
    obj.blob = NULL_SHA1
    assert obj.blob == NULL_SHA1


def test_fundamental_Tree_operations():
    tree = nog.Tree()
    tree.name = 'tree'
    tree.meta['foo'] = 'bar'
    tree.append(nog.Object(content=objContent_v0))
    tree.append(nog.Object(content=objContent_v1))
    assert tree.content == treeContent
    assert tree.sha1 == treeId

    remote.postTree(tree)
    tree = remote.getTree(treeId)
    assert tree.content == treeContent
    assert tree.sha1 == treeId

    tree.pop()
    tree.pop()
    tree.insert(0, nog.Object(content=objContent_v1))
    tree.insert(0, nog.Object(content=objContent_v0))
    assert tree.content == treeContent
    assert tree.sha1 == treeId

    tree.pop(0)
    tree.insert(0, nog.Object(content=objContent_v0))
    assert tree.content == treeContent
    assert tree.sha1 == treeId

    childTree = deepcopy(tree)
    tree.append(childTree)

    it = tree.entries()
    assert next(it).sha1 == objId_v0
    assert next(it).sha1 == objId_v1
    assert next(it).sha1 == treeId
    with pytest.raises(StopIteration):
        next(it)

    it = tree.enumerateEntries()
    (i, e) = next(it)
    assert i == 0
    assert e.sha1 == objId_v0
    (i, e) = next(it)
    assert i == 1
    assert e.sha1 == objId_v1
    (i, e) = next(it)
    assert i == 2
    assert e.sha1 == treeId
    with pytest.raises(StopIteration):
        next(it)

    it = tree.objects()
    assert next(it).sha1 == objId_v0
    assert next(it).sha1 == objId_v1
    with pytest.raises(StopIteration):
        next(it)

    it = tree.enumerateObjects()
    (i, e) = next(it)
    assert i == 0
    assert e.sha1 == objId_v0
    (i, e) = next(it)
    assert i == 1
    assert e.sha1 == objId_v1
    with pytest.raises(StopIteration):
        next(it)

    it = tree.trees()
    assert next(it).sha1 == treeId
    with pytest.raises(StopIteration):
        next(it)

    it = tree.enumerateTrees()
    (i, e) = next(it)
    assert i == 2
    assert e.sha1 == treeId
    with pytest.raises(StopIteration):
        next(it)


def test_Tree_collapse():
    tree = nog.Tree()
    obj = nog.Object()
    tree.append(obj)
    obj.name = 'foo'
    assert next(tree.objects()).name == 'foo'

    # Post tree and collapse entries, which detaches children.
    remote.postTree(tree)
    tree.collapse()

    obj.name = 'bar'
    assert next(tree.objects()).name == 'foo'


def test_fundamental_Commit_operations():
    bla = 'bla'
    blabla = 'bla bla...'

    # idversion 0 can be enforced when using UTC Z datetimes.
    id = remote.postCommitContent({
        '_idversion': 0,
        'subject': bla,
        'message': blabla,
        'tree': treeId,
        'parents': [],
        'authors': [AUThor],
        'authorDate': isoUTCZ,
        'committer': AUThor,
        'commitDate': isoUTCZ,
        'meta': {}
    })
    assert id == 'e9f56e990b7bf63a6068a78012fd9a423cbe5457'
    commit = remote.getCommit(id)
    assert commit.idversion == 0
    assert commit.subject == bla
    assert commit.message == blabla
    assert commit.tree.sha1 == treeId
    assert commit.authors == [AUThor]
    assert commit.authorDate == dateUTCZ
    assert commit.committer == AUThor
    assert commit.commitDate == dateUTCZ
    assert commit.meta == {}

    # idversion 1 is the default.
    id = remote.postCommitContent({
        'subject': bla,
        'message': blabla,
        'tree': treeId,
        'parents': [],
        'authors': [AUThor],
        'authorDate': isoUTCZ,
        'committer': AUThor,
        'commitDate': isoUTCZ,
        'meta': {}
    })
    assert id == '5419f596abd3de9cb2306d304278a39efa482f0a'
    commit = remote.getCommit(id)
    assert commit.idversion == 1

    id = remote.postCommitContent({
        'subject': bla,
        'message': blabla,
        'tree': treeId,
        'parents': [],
        'authors': [AUThor],
        'authorDate': isoTZPlus,
        'committer': AUThor,
        'commitDate': isoTZMinus,
        'meta': {}
    })
    assert id == 'd37d56e2b87fffd117857ec5d08c1ebf94f9522d'
    commit = remote.getCommit(id)
    assert commit.idversion == 1
    assert commit.subject == bla
    assert commit.message == blabla
    assert commit.tree.sha1 == treeId
    assert commit.authors == [AUThor]
    assert commit.authorDate == dateTZPlus
    assert commit.committer == AUThor
    assert commit.commitDate == dateTZMinus
    assert commit.meta == {}

    orig = remote.getMaster()
    remote.updateRef('branches/master', id, orig.sha1)
    tree = nog.Tree()
    tree.name = 'new name'
    remote.commitTree(subject=bla, tree=tree, parent=id)
    master = remote.getMaster()
    assert master.tree.sha1 == tree.sha1
    assert master.parents[0].sha1 == id
    assert master.idversion == 1


def test_object_with_special_character():
    master = remote.getMaster()
    root = master.tree
    oname = u'BlaBlub-\u00fc\u00e4\u00f6'
    tname = u'Wau-Wau-\u72d7'
    sname = u'Blafasel-\u0641\u0644\u0627\u0641\u0644'
    obj = nog.Object()
    obj.name = oname
    tr = nog.Tree()
    tr.name = tname
    tr.append(obj)
    root.append(tr)
    remote.commitTree(subject=sname, tree=root, parent=master.sha1)

    new_master = remote.getMaster()
    new_root = new_master.tree
    new_tr = next(new_root.trees(tname))
    new_obj = next(new_tr.objects(oname))
    assert new_obj.name == oname
    assert new_tr.name == tname
    assert new_master.subject == sname


def test_post_buf_size_flush():
    saved = nog.POST_BUFFER_SIZE
    savedLimit = nog.POST_BUFFER_SIZE_LIMIT
    nog.POST_BUFFER_SIZE = 1000
    nog.POST_BUFFER_SIZE_LIMIT = 100000
    obj = nog.Object()
    obj.text = 'a' * 100 * 100
    remote.postObject(obj)
    nog.POST_BUFFER_SIZE = saved
    nog.POST_BUFFER_SIZE_LIMIT = savedLimit


def test_post_buf_size_limit():
    saved = nog.POST_BUFFER_SIZE
    savedLimit = nog.POST_BUFFER_SIZE_LIMIT
    nog.POST_BUFFER_SIZE = 1000
    nog.POST_BUFFER_SIZE_LIMIT = 100000
    obj = nog.Object()
    obj.text = 'a' * 1000 * 1000
    with pytest.raises(RuntimeError):
        remote.postObject(obj)
    nog.POST_BUFFER_SIZE = saved
    nog.POST_BUFFER_SIZE_LIMIT = savedLimit
