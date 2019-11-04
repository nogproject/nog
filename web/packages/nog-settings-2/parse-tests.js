/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { createSettingsParser } from 'meteor/nog-settings-2';

function describeParseTests() {
  describe('settings parser', function () {
    const logger = {
      stdout: '',
      stderr: '',
      log(msg) {
        this.stdout += msg;
      },
      error(msg) {
        this.stderr += msg;
      },
      reset() {
        this.stdout = '';
        this.stderr = '';
      },
    };
    let parser = null;
    let settings = {};

    beforeEach(function () {
      logger.reset();
      parser = createSettingsParser({ logger });
      settings = {};
    });

    describe('defSetting() followed by parseSettings()', function () {
      it('handles dot paths.', function () {
        parser.defSetting({
          key: 'testing.foo', val: 1, help: 'foohelp', match: Number,
        });
        parser.defSetting({
          key: 'public.testing.bar', val: 2, help: 'barhelp', match: Number,
        });
        parser.parseSettings(settings);
        expect(settings.testing).to.be.a('object');
        expect(settings.testing.foo).to.equal(1);
        expect(settings.public.testing).to.be.a('object');
        expect(settings.public.testing.bar).to.equal(2);
      });

      it('preserves settings.', function () {
        settings.testing = {
          foo: 1,
          nil: null,
        };
        parser.defSetting({
          key: 'testing.foo', val: 2, help: 'foohelp', match: Number,
        });
        parser.defSetting({
          key: 'testing.nil', val: 3, help: 'nilhelp', match: null,
        });
        parser.parseSettings(settings);
        expect(settings.testing.foo).to.equal(1);
        expect(settings.testing.nil).to.equal(null);
      });

      it('sets default, reporting to console.', function () {
        parser.defSetting({
          logger,
          key: 'testing.foo', val: 1, help: 'foohelp', match: Number,
        });
        parser.parseSettings(settings);
        expect(logger.stdout).to.contain('Using default');
        expect(logger.stdout).to.contain('testing.foo=1');
      });

      it('sets default array, reporting to console.', function () {
        parser.defSetting({
          logger,
          key: 'testing.arr', val: [1, 2], help: 'arrhelp', match: [Number],
        });
        parser.parseSettings(settings);
        expect(logger.stdout).to.contain('testing.arr=[1,2]');
      });

      it('sets default object, reporting to console.', function () {
        parser.defSetting({
          logger,
          key: 'testing.obj', val: { a: 1 }, help: 'objhelp', match: Object,
        });
        parser.parseSettings(settings);
        expect(logger.stdout).to.contain('testing.obj={"a":1}');
      });

      it('checks value, reporting to console.', function () {
        parser.defSetting({
          logger,
          key: 'testing.foo', val: { a: 1 }, help: 'foohelp', match: Number,
        });
        const fn = () => parser.parseSettings(settings);
        expect(fn).to.throw('Match error');
        expect(logger.stderr).to.contain('Invalid setting');
        expect(logger.stderr).to.contain('testing.foo={"a":1}');
        expect(logger.stderr).to.contain('foohelp');
        expect(logger.stderr).to.contain('Match error');
        expect(fn).to.throw('Expected number');
        expect(fn).to.throw('got object');
      });
    });

    describe('settingsUsage()', function () {
      it('describes defSetting() details.', function () {
        parser.defSetting({
          key: 'testing.foo', val: 2, help: 'foohelp', match: Number,
        });
        parser.defSetting({
          key: 'testing.nil', val: 3, help: 'nilhelp', match: null,
        });
        const usage = parser.settingsUsage();
        expect(usage).to.have.string('settings.testing.foo');
        expect(usage).to.have.string('foohelp');
        expect(usage).to.have.string('settings.testing.nil');
        expect(usage).to.have.string('nilhelp');
      });
    });
  });
}

export {
  describeParseTests,
};
