import { expect } from 'chai'

{nogthrow} = NogError

describe 'nog-rest', ->
  describe 'NogRest.actions', ->

    fakeStatusCode = 404
    fakeErrorCode = 'FAKE_ERROR_CODE'
    actionParams = null
    actionBaseUrl = null
    actionQuery = null
    redirectUrl = 'http://example.com/'
    postBody = null

    get_action =
      method: 'GET'
      path: '/:id'
      action: (req) ->
        {params, baseUrl} = req
        actionParams = params
        actionBaseUrl = baseUrl
        actionQuery = req.query
        if params.id is 'statusCode'
          err = new Meteor.Error fakeErrorCode, 'fakeError'
          err.statusCode = fakeStatusCode
          throw err
        if params.id is 'checkError'
          check params, { id: Number, missing: String }
        if params.id is 'jsError'
          throw new Error 'generic JavaScript Error'
        if params.id is 'redirect'
          return {
            statusCode: 307
            location: redirectUrl
            message: "Please re-send the request to the URL in 'location'. " +
              " Continue to use the original URL for future requests."
          }
        if params.id is 'status201'
          return _.extend {statusCode: 201}, _.pick(params, 'tag', 'id')
        return _.pick params, 'tag', 'id'

    post_action =
      method: 'POST'
      path: '/:id'
      action: (req) ->
        postBody = req.body
        return {}

    get_root_action =
      method: 'GET'
      path: '/'
      action: (req) ->
        actionParams = req.params
        actionBaseUrl = req.baseUrl
        actionQuery = req.query

    origConfig =
      nogRest: null
    before ->
      origConfig.nogRest = _.pick NogRest, 'checkRequestAuth'
      NogRest.configure { checkRequestAuth: -> }
    after ->
      NogRest.configure origConfig.nogRest

    prefix = null

    it 'installs action with {baseUrl, genBaseUrl}.', ->
      prefix = '/' + Random.id()
      NogRest.actions prefix + '/:tag', [get_action, post_action]

      res = HTTP.get(Meteor.absoluteUrl("/#{prefix}/a/1"))
      expect(res.statusCode).to.equal 200

    it 'The action `/` matches at the root.', ->
      prefix2 = '/' + Random.id()
      NogRest.actions prefix2, [get_root_action]
      actionQuery = null
      actionBaseUrl = null
      res = HTTP.get Meteor.absoluteUrl("/#{prefix2}?foo=bar")
      expect(res.statusCode).to.equal 200
      expect(actionQuery.foo).to.equal 'bar'
      expect(actionBaseUrl).to.equal prefix2

    it 'The action is called with a parsed body.', ->
      fakeData = {fake: 'foo'}
      res = HTTP.post Meteor.absoluteUrl("/#{prefix}/a/1"), {data: fakeData}
      expect(postBody).to.deep.equal fakeData

    it 'An action is ignored for an unknown HTTP method.', ->
      res = HTTP.del(Meteor.absoluteUrl("/#{prefix}/a/1"))
      expect(res.data).to.not.exist

    it 'passes the params and query to the action.', ->
      actionParams = null
      actionQuery = null
      res = HTTP.get(Meteor.absoluteUrl("/#{prefix}/a/1?foo=bar"))
      expect(actionParams.tag).to.equal 'a'
      expect(actionParams.id).to.equal '1'
      expect(actionQuery.foo).to.equal 'bar'

    it 'passes the correct baseUrl if it depends on params.', ->
      actionBaseUrl = null
      res = HTTP.get(Meteor.absoluteUrl("/#{prefix}/a/1"))
      expect(actionBaseUrl).to.equal prefix + '/a'

    it 'passes the correct baseUrl if it does not depend on params.', ->
      prefix2 = '/' + Random.id()
      NogRest.actions prefix2, [get_action]
      actionBaseUrl = null
      res = HTTP.get(Meteor.absoluteUrl("/#{prefix2}/1"))
      expect(actionBaseUrl).to.equal prefix2

    it 'returns a JSON result for a successful action.', ->
      res = HTTP.get(Meteor.absoluteUrl("/#{prefix}/a/1"))
      expect(res.statusCode).to.equal 200
      expect(res.data.statusCode).to.equal 200
      expect(res.data.data.tag).to.equal 'a'
      expect(res.data.data.id).to.equal '1'

    it 'returns a 2xx statusCode from a successful action.', ->
      res = HTTP.get(Meteor.absoluteUrl("/#{prefix}/a/status201"))
      expect(res.statusCode).to.equal 201
      expect(res.data.statusCode).to.equal 201
      expect(res.data.data.statusCode).to.not.exist
      expect(res.data.data.tag).to.equal 'a'
      expect(res.data.data.id).to.equal 'status201'

    # Use HTTP async to inspect the response body.
    it 'returns the error if the action throws.', (done) ->
      HTTP.get Meteor.absoluteUrl("/#{prefix}/a/statusCode"), (err, res) ->
        expect(err).to.exist
        expect(res.statusCode).to.equal fakeStatusCode
        expect(res.data.statusCode).to.equal fakeStatusCode
        expect(res.data.errorCode).to.equal fakeErrorCode
        expect(res.data.message).to.exist
        done()

    it "reports a check error as '422 Unprocessable Entity'.", (done) ->
      HTTP.get Meteor.absoluteUrl("/#{prefix}/a/checkError"), (err, res) ->
        expect(err).to.exist
        expect(res.statusCode).to.equal 422
        expect(res.data.statusCode).to.equal 422
        expect(res.data.errorCode).to.contain 'MATCH'
        expect(res.data.message).to.exist
        done()

    it "handles generic JavaScript errors.", (done) ->
      HTTP.get Meteor.absoluteUrl("/#{prefix}/a/jsError"), (err, res) ->
        expect(err).to.exist
        expect(res.statusCode).to.equal 500
        expect(res.data.statusCode).to.equal 500
        expect(res.data.errorCode).to.contain 'UNEXPECTED_EXCEPTION'
        expect(res.data.message).to.exist
        done()

    it 'action timeout is reported as 503', (done) ->
      pfx = '/' + Random.id()
      get_timeout_action = {
        method: 'GET'
        path: ''
        action: -> Meteor._sleepForMs 5 * 1000
      }
      NogRest.actions pfx, [get_timeout_action]
      origcfg = NogRest.configure {timeout_s: 0.01}
      res = HTTP.get Meteor.absoluteUrl("/#{pfx}"), (err, res) ->
        NogRest.configure origcfg
        expect(err).to.exist
        expect(res.statusCode).to.equal 503
        expect(res.data.errorCode).to.contain 'PROC_TIMEOUT'
        done()

    it "handles redirect results.", (done) ->
      HTTP.get Meteor.absoluteUrl("/#{prefix}/a/redirect"), {
        followRedirects: false
      }, (err, res) ->
        expect(err).to.not.exist
        expect(res.statusCode).to.equal 307
        expect(res.headers.location).to.equal redirectUrl
        done()

    it "calls checkRequestAuth() with the request.", ->
      authReq = null
      NogRest.configure
        checkRequestAuth: (req) -> authReq = req
      HTTP.get(Meteor.absoluteUrl("/#{prefix}/a/1"))
      expect(authReq).to.exist

    it "reports an auth failure with statusCode 401.", (done) ->
      NogRest.configure
        checkRequestAuth: (req) -> nogthrow
          errorCode: 'FAKE'
          statusCode: 401
      HTTP.get Meteor.absoluteUrl("/#{prefix}/a/1"), (err, res) ->
        expect(err).to.exist
        expect(res.statusCode).to.equal 401
        done()
