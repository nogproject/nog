import { expect } from 'chai'
{createError, nogthrow, defaultErrorHandler} = NogError

# Add helper to define test only on client.
it.client = (args...) -> if Meteor.isClient then @(args...)


fakeErrorCode = '999999'

describe 'nog-error', ->
  fakeSpec = (diff) ->
    _.extend {
      errorCode: 'INVALID'
      statusCode: 0
      sanitized: null
      reason: ''
      details: ''
    }, diff

  describe 'createError(spec, context)', ->
    it 'uses errorCode from spec.', ->
      e = createError fakeSpec {errorCode: 'FAKE'}
      expect(e.errorCode).to.equal 'FAKE'

    it 'uses statusCode from spec.', ->
      e = createError fakeSpec {statusCode: 555}
      expect(e.statusCode).to.equal 555

    it 'uses reason from spec.', ->
      e = createError fakeSpec {reason: 'foo'}
      expect(e.reason).to.contain 'foo'

    it 'uses reason from spec (function).', ->
      e = createError fakeSpec({reason: (ctx) -> ctx.foo}), {
          foo: 'foo'
        }
      expect(e.reason).to.contain 'foo'
      expect(e.reason).to.not.contain 'function'

    it 'uses details from spec.', ->
      e = createError fakeSpec {details: 'foo'}
      expect(e.details).to.contain 'foo'

    it 'uses details from spec (function).', ->
      e = createError fakeSpec({details: (ctx) -> ctx.foo}), {
          foo: 'foo'
        }
      expect(e.details).to.contain 'foo'
      expect(e.details).to.not.contain 'function'

    it 'uses sanitized defaults.', ->
      {sanitizedError} = createError fakeSpec
        sanitized: null
        reason: 'foo'
        details: 'bar'
      expect(sanitizedError.error).to.equal 'NOGERR'
      expect(sanitizedError.reason).to.contain 'Unspecified'
      expect(sanitizedError.reason).to.not.contain 'foo'
      expect(sanitizedError.details).to.not.contain 'bar'
      expect(sanitizedError.message).to.contain 'NOGERR'
      expect(sanitizedError.message).to.contain 'Unspecified'

    it 'uses sanitized object from spec.', ->
      {sanitizedError} = createError fakeSpec
        sanitized:
          errorCode: 'FAKE'
          reason: 'foo'
          details: 'bar'
      expect(sanitizedError.error).to.equal 'FAKE'
      expect(sanitizedError.reason).to.contain 'foo'
      expect(sanitizedError.reason).to.not.contain 'Unspecified'
      expect(sanitizedError.details).contain 'bar'
      expect(sanitizedError.message).to.contain 'FAKE'
      expect(sanitizedError.message).to.contain 'foo'

    it 'uses sanitized object from spec (functions).', ->
      {sanitizedError} = createError fakeSpec(
          sanitized:
            errorCode: 'FAKE'
            reason: (ctx) -> ctx.foo
            details: (ctx) -> ctx.bar
        ), {
          foo: 'foo'
          bar: 'bar'
        }
      expect(sanitizedError.reason).to.contain 'foo'
      expect(sanitizedError.reason).to.not.contain 'Unspecified'
      expect(sanitizedError.details).contain 'bar'
      expect(sanitizedError.message).to.contain 'foo'

    it 'supports sanitized `full`.', ->
      {sanitizedError} = createError fakeSpec({sanitized: 'full'}),
          reason: 'foo'
          details: 'bar'
      expect(sanitizedError.reason).to.contain 'foo'
      expect(sanitizedError.message).to.contain 'foo'
      expect(sanitizedError.reason).to.not.contain 'Unspecified'
      expect(sanitizedError.details).contain 'bar'

    it 'uses reason from context.', ->
      e = createError fakeSpec(),
          reason: 'foo'
      expect(e.reason).to.contain 'foo'

    it 'uses details from context.', ->
      e = createError fakeSpec(),
          details: 'bar'
      expect(e.details).to.contain 'bar'

    it 'adds time.', ->
      e = createError fakeSpec()
      expect(e.time).to.exist

    it 'adds a token.', ->
      e = createError fakeSpec()
      expect(e.token).to.exist

    it 'stores a context object.', ->
      e = createError fakeSpec(),
          foo: 'foo'
      expect(e.context.foo).to.equal 'foo'

    it 'composes the reason from cause.', ->
      cause = new Meteor.Error 'FAKE', 'foo', 'bar'
      e = createError fakeSpec(), {cause}
      expect(e.reason).to.contain 'foo'

    it 'composes the details from cause.', ->
      cause = new Meteor.Error 'FAKE', 'foo', 'bar'
      e = createError fakeSpec(), {cause}
      expect(e.details).to.contain 'bar'

    it 'composes the sanitized reason from cause.', ->
      cause = createError fakeSpec({sanitized: 'full'}), {
          reason: 'foo', details: 'bar'
        }
      {sanitizedError} = createError fakeSpec(), {cause}
      expect(sanitizedError.reason).to.contain 'foo'

    it 'composes the sanitized details from cause.', ->
      cause = createError fakeSpec({sanitized: 'full'}), {
          reason: 'foo', details: 'bar'
        }
      {sanitizedError} = createError fakeSpec(), {cause}
      expect(sanitizedError.details).to.contain 'bar'

    it 'composes the sanitized reason from cause (full).', ->
      cause = createError fakeSpec(), {
          reason: 'foo', details: 'bar'
        }
      {sanitizedError} = createError fakeSpec({sanitized: 'full'}), {cause}
      expect(sanitizedError.reason).to.contain 'foo'

    it 'composes the sanitized details from cause (full).', ->
      cause = createError fakeSpec(), {
          reason: 'foo', details: 'bar'
        }
      {sanitizedError} = createError fakeSpec({sanitized: 'full'}), {cause}
      expect(sanitizedError.details).to.contain 'bar'

    it 'adds the error to the error log collection.', ->
      e = createError fakeSpec {errorCode: 'FAKE'}
      l = NogError.errorLog.findOne _.pick e, 'token', 'time'
      expect(l).to.exist

  describe 'nogthrow()', ->
    it 'throws for (spec, context).', ->
      fn = -> nogthrow fakeSpec({errorCode: 'FAKE'}), {reason: 'foo'}
      expect(fn).to.throw 'FAKE'
      expect(fn).to.throw 'foo'

    it 'throws for legacy (code, reason, details).', ->
      fn = -> nogthrow(fakeErrorCode, 'reason', 'details')
      expect(fn).to.throw fakeErrorCode
      expect(fn).to.throw 'reason'

  describe 'legacy createError(code, reason, details)', ->
    it 'accepts a string as details.', ->
      e = createError fakeErrorCode, 'reason', 'details'
      expect(e.errorCode).to.be.equal fakeErrorCode
      expect(e.statusCode).to.be.equal 500
      expect(e.reason).to.contain 'reason'
      expect(e.details).to.contain 'details'
      expect(e.history[0].details).to.contain 'details'
      expect(e.message).to.contain fakeErrorCode
      expect(e.message).to.contain 'reason'
      expect(e.sanitizedError.message).to.contain fakeErrorCode
      expect(e.sanitizedError.message).to.contain 'reason'
      expect(e.sanitizedError.details).to.contain 'details'

    it 'accepts undefined details.', ->
      e = createError fakeErrorCode, 'reason'
      expect(e.errorCode).to.be.equal fakeErrorCode
      expect(e.reason).to.be.contain 'reason'
      expect(e.message).to.contain fakeErrorCode
      expect(e.message).to.contain 'reason'
      expect(e.sanitizedError.message).to.contain fakeErrorCode
      expect(e.sanitizedError.message).to.contain 'reason'

    it 'accepts an Error as details.', ->
      details = new Error 'origError'
      e = createError fakeErrorCode, 'reason', details
      expect(e.message).to.contain 'origError'
      expect(e.history[1].reason).to.contain 'origError'
      expect(e.sanitizedError.message).to.contain 'origError'

    it 'accepts a Meteor.Error as details.', ->
      details = createError 'err1', 'reason1', 'details1'
      e = createError fakeErrorCode, 'reason', details
      expect(e.history[1].errorCode).to.equal 'err1'
      expect(e.history[1].reason).to.contain 'reason1'
      expect(e.history[1].details).to.contain 'details1'
      expect(e.sanitizedError.message).to.contain 'reason1'
      expect(e.sanitizedError.details).to.contain 'details1'

  describe 'defaultErrorHandler()', ->
    it.client "sets Session 'errors'.", ->
      Session.set 'errors', null
      defaultErrorHandler createError(fakeErrorCode, 'reason', 'details')
      expect(Session.get('errors')[0].errorCode).to.be.equal fakeErrorCode

  describe 'errorDisplay', ->
    it.client "reacts to Session 'errors'.", ->
      Session.set 'errors', null
      tmpl = Template.errorDisplay
      errorDisplay = $(renderToDiv tmpl).find('.nog-error-display')
      expect(errorDisplay.length).to.equal 0
      defaultErrorHandler createError(fakeErrorCode, 'reason', 'details')
      errorDisplay = $(renderToDiv tmpl).find('.nog-error-display')
      expect(errorDisplay.length).to.equal 1

    it.client "clears Session 'errors' when clicking the clear button.", ->
      Session.set 'errors', null
      defaultErrorHandler createError(fakeErrorCode, 'reason', 'details')
      expect(Session.get 'errors').to.have.length 1
      tmpl = Template.errorDisplay
      $(renderToDiv tmpl).find('.js-clear-errors').click()
      expect(Session.get 'errors').to.not.exist
