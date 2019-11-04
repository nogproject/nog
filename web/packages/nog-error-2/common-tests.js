/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { Meteor } from 'meteor/meteor';

import { nogthrow, createErrorModule } from 'meteor/nog-error-2';

function fakeSpec(patch) {
  const spec = {
    errorCode: 'INVALID',
    statusCode: 0,
    sanitized: null,
    reason: '',
    details: '',
  };
  return Object.assign({}, spec, patch);
}

describe('nog-error-2', function () {
  describe('createError()', function () {
    const NogError = createErrorModule({
      platform: { where: 'testing' },
    });
    const { createError } = NogError;

    it('uses errorCode from spec.', function () {
      const s = fakeSpec({ errorCode: 'FAKE' });
      const e = createError(s);
      expect(e.errorCode).to.equal('FAKE');
    });

    it('uses statusCode from spec.', function () {
      const s = fakeSpec({ statusCode: 555 });
      const e = createError(s);
      expect(e.statusCode).to.equal(555);
    });

    it('uses reason from spec.', function () {
      const s = fakeSpec({ reason: 'foo' });
      const e = createError(s);
      expect(e.reason).to.contain('foo');
    });

    it('uses reason from spec (function).', function () {
      const s = fakeSpec({
        reason: ctx => ctx.foo,
      });
      const e = createError(s, {
        foo: 'foo',
      });
      expect(e.reason).to.contain('foo');
      expect(e.reason).to.not.contain('function');
    });

    it('uses details from spec.', function () {
      const s = fakeSpec({ details: 'foo' });
      const e = createError(s);
      expect(e.details).to.contain('foo');
    });

    it('uses details from spec (function).', function () {
      const s = fakeSpec({
        details: ctx => ctx.foo,
      });
      const e = createError(s, {
        foo: 'foo',
      });
      expect(e.details).to.contain('foo');
      expect(e.details).to.not.contain('function');
    });

    it('uses sanitized defaults.', function () {
      const s = fakeSpec({
        sanitized: null,
        reason: 'foo',
        details: 'bar',
      });
      const { sanitizedError } = createError(s);
      expect(sanitizedError.error).to.equal('NOGERR');
      expect(sanitizedError.reason).to.contain('Unspecified');
      expect(sanitizedError.reason).to.not.contain('foo');
      expect(sanitizedError.details).to.not.contain('bar');
      expect(sanitizedError.message).to.contain('NOGERR');
      expect(sanitizedError.message).to.contain('Unspecified');
    });

    it('uses sanitized object from spec.', function () {
      const s = fakeSpec({
        sanitized: {
          errorCode: 'FAKE',
          reason: 'foo',
          details: 'bar',
        },
      });
      const { sanitizedError } = createError(s);
      expect(sanitizedError.error).to.equal('FAKE');
      expect(sanitizedError.reason).to.contain('foo');
      expect(sanitizedError.reason).to.not.contain('Unspecified');
      expect(sanitizedError.details).contain('bar');
      expect(sanitizedError.message).to.contain('FAKE');
      expect(sanitizedError.message).to.contain('foo');
    });

    it('uses sanitized object from spec (functions).', function () {
      const s = fakeSpec({
        sanitized: {
          errorCode: 'FAKE',
          reason: ctx => ctx.foo,
          details: ctx => ctx.bar,
        },
      });
      const { sanitizedError } = createError(s, {
        foo: 'foo',
        bar: 'bar',
      });
      expect(sanitizedError.reason).to.contain('foo');
      expect(sanitizedError.reason).to.not.contain('Unspecified');
      expect(sanitizedError.details).contain('bar');
      expect(sanitizedError.message).to.contain('foo');
    });

    it('supports sanitized `full`.', function () {
      const s = fakeSpec({ sanitized: 'full' });
      const { sanitizedError } = createError(s, {
        reason: 'foo',
        details: 'bar',
      });
      expect(sanitizedError.reason).to.contain('foo');
      expect(sanitizedError.message).to.contain('foo');
      expect(sanitizedError.reason).to.not.contain('Unspecified');
      expect(sanitizedError.details).contain('bar');
    });

    it('uses reason from context.', function () {
      const s = fakeSpec();
      const e = createError(s, {
        reason: 'foo',
      });
      expect(e.reason).to.contain('foo');
    });

    it('uses details from context.', function () {
      const s = fakeSpec();
      const e = createError(s, {
        details: 'bar',
      });
      expect(e.details).to.contain('bar');
    });

    it('adds time.', function () {
      const s = fakeSpec();
      const e = createError(s);
      expect(e).to.have.property('time').that.is.instanceof(Date);
    });

    it('adds a token.', function () {
      const s = fakeSpec();
      const e = createError(s);
      expect(e).to.have.property('token').that.is.a('string');
    });

    it('stores a context object.', function () {
      const s = fakeSpec();
      const e = createError(s, {
        foo: 'foo',
      });
      expect(e.context.foo).to.equal('foo');
    });

    it('composes the reason from cause.', function () {
      const cause = new Meteor.Error('FAKE', 'foo', 'bar');
      const s = fakeSpec();
      const e = createError(s, { cause });
      expect(e.reason).to.contain('foo');
    });

    it('composes the details from cause.', function () {
      const cause = new Meteor.Error('FAKE', 'foo', 'bar');
      const s = fakeSpec();
      const e = createError(s, { cause });
      expect(e.details).to.contain('bar');
    });

    it('composes the sanitized reason from cause.', function () {
      const causeSpec = fakeSpec({ sanitized: 'full' });
      const cause = createError(causeSpec, {
        reason: 'foo',
        details: 'bar',
      });
      const s = fakeSpec();
      const { sanitizedError } = createError(s, { cause });
      expect(sanitizedError.reason).to.contain('foo');
    });

    it('composes the sanitized details from cause.', function () {
      const causeSpec = fakeSpec({ sanitized: 'full' });
      const cause = createError(causeSpec, {
        reason: 'foo',
        details: 'bar',
      });
      const s = fakeSpec();
      const { sanitizedError } = createError(s, { cause });
      expect(sanitizedError.details).to.contain('bar');
    });

    it('composes the sanitized reason from cause (full).', function () {
      const causeSpec = fakeSpec();
      const cause = createError(causeSpec, {
        reason: 'foo',
        details: 'bar',
      });
      const s = fakeSpec({ sanitized: 'full' });
      const { sanitizedError } = createError(s, { cause });
      expect(sanitizedError.reason).to.contain('foo');
    });

    it('composes the sanitized details from cause (full).', function () {
      const causeSpec = fakeSpec();
      const cause = createError(causeSpec, {
        reason: 'foo',
        details: 'bar',
      });
      const s = fakeSpec({ sanitized: 'full' });
      const { sanitizedError } = createError(s, { cause });
      expect(sanitizedError.details).to.contain('bar');
    });

    // Do not test `it('adds the error to the error log collection.')` as in
    // `nog-error`, because the log collection in optional in `nog-error-2`;
  });

  describe('nogthrow()', function () {
    it('throws for (spec, context).', function () {
      function fn() {
        const s = fakeSpec({ errorCode: 'FAKE' });
        nogthrow(s, { reason: 'foo' });
      }
      expect(fn).to.throw('FAKE');
      expect(fn).to.throw('foo');
    });
  });
});
