import chai from 'chai'
import sinon from 'sinon'
import sinonChai from 'sinon-chai'
chai.use(sinonChai)
{ expect } = chai
{ spy } = sinon

{
  CachedRepoSets
  TransientSet
} = require './nog-content-caching-server.coffee'

pause = (duration_ms, fn) -> setTimeout fn, duration_ms


describe 'nog-content', -> describe 'TransientSet', ->
  it 'maintains a set', ->
    ts = new TransientSet()

    ts.insert 'a'
    expect(ts.contains('a')).to.be.true
    expect(ts.size()).to.eql 1

    ts.insert 'a'
    expect(ts.size()).to.eql 1

    ts.insert 'b'
    expect(ts.contains('b')).to.be.true
    expect(ts.size()).to.eql 2

    ts.clear()
    expect(ts.contains('a')).to.be.false
    expect(ts.size()).to.eql 0

  it 'max size can be configured', ->
    ts = new TransientSet {maxSize: 1}
    ts.insert 'a'
    ts.insert 'b'
    expect(ts.contains('a')).to.be.false

  it 'max age can be configured', (done) ->
    ts = new TransientSet {maxAge_s: 1}
    ts.insert 'a'
    expect(ts.contains('a')).to.be.true
    pause 1200, ->
      expect(ts.contains('a')).to.be.false
      done()


describe 'nog-content', -> describe 'CachedRepoSets', ->
  trueSet =
    isMember: spy -> true
    updateMembership: spy ->
    checkMembership: spy ->

  falseSet =
    isMember: spy -> false
    updateMembership: spy ->
    checkMembership: spy -> throw new Error('testing error')

  arg = {
    ownerName: 'fred', repoName: 'bar',
    sha1: 'babababababababababababababababababababa'
  }
  arg2 = {
    ownerName: 'fred', repoName: 'bar',
    sha1: 'fefefefefefefefefefefefefefefefefefefefe'
  }

  entry = {type: 'object', sha1: arg.sha1}

  it 'caches isMember', ->
    rs = new CachedRepoSets trueSet
    trueSet.isMember.reset()
    expect(rs.isMember(arg)).to.be.true
    expect(rs.isMember(arg)).to.be.true
    expect(trueSet.isMember).to.have.been.calledOnce

    rs = new CachedRepoSets falseSet
    falseSet.isMember.reset()
    expect(rs.isMember(arg)).to.be.false
    expect(rs.isMember(arg)).to.be.false
    expect(falseSet.isMember).to.have.been.calledTwice

  it 'caches checkMembership', ->
    rs = new CachedRepoSets trueSet
    trueSet.checkMembership.reset()
    rs.checkMembership(arg)  # Does not throw.
    rs.checkMembership(arg)  # Does not throw.
    expect(trueSet.checkMembership).to.have.been.calledOnce

    rs = new CachedRepoSets falseSet
    falseSet.checkMembership.reset()
    fn = -> rs.checkMembership(arg)
    expect(fn).to.throw 'testing error'
    expect(fn).to.throw 'testing error'
    expect(falseSet.isMember).to.have.been.calledTwice

  it 'caches effect of updateMembership', ->
    rs = new CachedRepoSets trueSet
    trueSet.checkMembership.reset()
    trueSet.updateMembership.reset()
    rs.updateMembership(arg, entry)
    expect(rs.isMember(arg)).to.be.true
    expect(trueSet.updateMembership).to.have.been.calledOnce
    expect(trueSet.checkMembership).to.have.not.been.called

  it 'cache size can be configured', ->
    rs = new CachedRepoSets trueSet, {maxCacheSize: 1}
    for i in [0..2]
      trueSet.isMember.reset()
      expect(rs.isMember(arg)).to.be.true
      expect(rs.isMember(arg2)).to.be.true
      expect(trueSet.isMember).to.have.been.calledTwice

  it 'cache age can be configured', (done) ->
    rs = new CachedRepoSets trueSet, {maxCacheAge_s: 1}
    fn = ->
      trueSet.isMember.reset()
      expect(rs.isMember(arg)).to.be.true
      expect(rs.isMember(arg2)).to.be.true
      expect(trueSet.isMember).to.have.been.calledTwice
    fn()
    pause 1200, ->
      fn()
      done()
