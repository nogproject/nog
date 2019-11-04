/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */

import { expect } from 'chai';
import { Meteor } from 'meteor/meteor';
import { defSetting } from 'meteor/nog-settings';


describe('nog-settings', function () {
  describe('defSetting', function () {
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

    beforeEach(function () {
      logger.reset();
      delete Meteor.settings.testing;
      delete Meteor.settings.public.testing;
    });

    it('Handles dot paths.', function () {
      defSetting({
        key: 'testing.foo', val: 1, help: 'foohelp', match: Number,
      });
      defSetting({
        key: 'public.testing.bar', val: 2, help: 'barhelp', match: Number,
      });

      expect(Meteor.settings.testing).to.exist;
      expect(Meteor.settings.testing.foo).to.eql(1);
      expect(Meteor.settings.public.testing).to.exist;
      expect(Meteor.settings.public.testing.bar).to.eql(2);
    });

    it('Keeps settings.', function () {
      Meteor.settings.testing = {
        foo: 1,
        nil: null,
      };

      defSetting({
        key: 'testing.foo', val: 2, help: 'foohelp', match: Number,
      });
      defSetting({
        key: 'testing.nil', val: 3, help: 'nilhelp', match: null,
      });

      expect(Meteor.settings.testing.foo).to.eql(1);
      expect(Meteor.settings.testing.nil).to.eql(null);
    });

    it('Sets default, reporting to console.', function () {
      defSetting({
        logger,
        key: 'testing.foo', val: 1, help: 'foohelp', match: Number,
      });

      expect(logger.stdout).to.contain('Using default');
      expect(logger.stdout).to.contain('testing.foo=1');
    });

    it('Sets default array, reporting to console.', function () {
      defSetting({
        logger,
        key: 'testing.arr', val: [1, 2], help: 'arrhelp', match: [Number],
      });

      expect(logger.stdout).to.contain('testing.arr=[1,2]');
    });

    it('Sets default object, reporting to console.', function () {
      defSetting({
        logger,
        key: 'testing.obj', val: { a: 1 }, help: 'objhelp', match: Object,
      });

      expect(logger.stdout).to.contain('testing.obj={"a":1}');
    });

    it('Checks value, reporting to console.', function () {
      const fn = () => defSetting({
        logger,
        key: 'testing.foo', val: { a: 1 }, help: 'foohelp', match: Number,
      });

      expect(fn).to.throw('Match error');
      expect(logger.stderr).to.contain('Invalid setting');
      expect(logger.stderr).to.contain('testing.foo={"a":1}');
      expect(logger.stderr).to.contain('foohelp');
      expect(logger.stderr).to.contain('Match error');
      expect(fn).to.throw('Expected number');
      expect(fn).to.throw('got object');
    });
  });
});
