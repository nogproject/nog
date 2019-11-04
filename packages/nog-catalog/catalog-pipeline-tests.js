/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */

import { check } from 'meteor/check';
import { expect } from 'chai';
import { _ } from 'meteor/underscore';

import {
  matchCatalogPipelineStep,
  compilePipeline,
} from './catalog-pipeline.js';


const allPipelineOps = [
  {
    $select: { foo: 1 },
  },
  {
    $addStaticLabels: { labels: { foo: 'bar' } },
  },
  {
    $updateName: {
      $select: { 'meta.foo': { $exists: true } },
      $set: { $mustache: '{{{meta.bar}}}' },
    },
  },
  {
    $updateMeta: {
      $select: { 'meta.foo': { $exists: true } },
      $set: { bar: 1 },
    },
  },
  {
    $updateMeta: {
      $select: { 'meta.foo': { $exists: true } },
      $addToSet: { bar: 1 },
    },
  },
  {
    $updateMeta: {
      $select: { 'meta.foo': { $exists: true } },
      $unset: { bar: '' },
    },
  },
  {
    $updateMeta: {
      $select: { 'meta.foo': { $exists: true } },
      $rename: { bar: 'baz' },
    },
  },
  {
    $updateMeta: {
      $select: { 'meta.keywords': { $exists: true } },
      $set: {
        keywords: {
          $splitField: {
            field: 'meta.keywords', separator: ',', trim: true,
          },
        },
      },
    },
  },
];


describe('catalog-pipeline', function () {
  describe('matchCatalogPipelineStep()', function () {
    it('accepts valid pipeline', function () {
      const steps = allPipelineOps;
      for (const s of steps) {
        check(s, matchCatalogPipelineStep);
      }
    });

    it('rejects invalid pipeline', function () {
      const fn = () => check({ $invalid: {} }, matchCatalogPipelineStep);
      expect(fn).to.throw('Match error');
    });
  });

  describe('compilePipeline()', function () {
    it('compiles pipeline', function () {
      const pipe = allPipelineOps;
      const funcs = compilePipeline(pipe);
      expect(funcs.length).to.eql(pipe.length);
      for (const fn of funcs) {
        expect(_.isFunction(fn)).to.be.true;
      }
    });
  });

  describe('$select', function () {
    const [$select] = compilePipeline([
      { $select: { 'meta.foo': { $exists: true } } },
    ]);

    it('accepts content', function () {
      expect($select({ meta: { foo: 1 } })).to.exist;
    });

    it('rejects content', function () {
      expect($select({ meta: { bar: 1 } })).to.not.exist;
    });
  });

  describe('$addStaticLabels', function () {
    const [$addStaticLabels] = compilePipeline([
      { $addStaticLabels: { labels: { foo: 'x', bar: 'y' } } },
    ]);

    it('adds labels', function () {
      expect($addStaticLabels({}).meta.foo).to.eql('x');
      expect($addStaticLabels({}).meta.bar).to.eql('y');
    });
  });

  describe('$updateName', function () {
    const [$updateName] = compilePipeline([
      { $updateName: { $set: { $mustache: '{{{meta.foo}}}' } } },
    ]);

    const [$updateNameSelect] = compilePipeline([
      {
        $updateName: {
          $select: { 'meta.foo': { $exists: true } },
          $set: { $mustache: '{{{meta.bar}}}' },
        },
      },
    ]);

    it('updates name', function () {
      const o = $updateName({ meta: { foo: 'fooval' } });
      expect(o.name).to.eql('fooval');
    });

    it('updates name if select', function () {
      const o = $updateNameSelect(
        { meta: { foo: 'fooval', bar: 'barval' } },
      );
      expect(o.name).to.eql('barval');
    });

    it('leaves name if not select', function () {
      const o = $updateNameSelect(
        { name: 'orig', meta: { bar: 'barval' } },
      );
      expect(o.name).to.eql('orig');
    });
  });

  describe('$updateMeta', function () {
    describe('{ $select ... }', function () {
      const [$updateMeta] = compilePipeline([
        {
          $updateMeta: {
            $select: { 'meta.foo': { $exists: true } },
            $set: { bar: 'barval' },
          },
        },
      ]);

      it('accepts', function () {
        const o = $updateMeta({ meta: { foo: 1 } });
        expect(o.meta.bar).to.eql('barval');
      });

      it('rejects', function () {
        expect($updateMeta({ meta: {} }).meta.bar).to.not.exist;
      });
    });

    describe('{ $set ... }', function () {
      const [$updateMeta] = compilePipeline([
        { $updateMeta: { $set: { bar: 'newbar' } } },
      ]);

      it('sets meta', function () {
        const o = $updateMeta({ meta: { bar: 'oldbar' } });
        expect(o.meta.bar).to.eql('newbar');
      });
    });

    describe('{ $addToSet ... }', function () {
      const [$updateMeta] = compilePipeline([
        { $updateMeta: { $addToSet: { bar: 'bar2' } } },
      ]);

      it('adds meta', function () {
        let o = $updateMeta({ meta: { bar: ['bar1'] } });
        expect(o.meta.bar).to.eql(['bar1', 'bar2']);
        o = $updateMeta(o);
        expect(o.meta.bar).to.eql(['bar1', 'bar2']);
      });
    });

    describe('{ $unset ... }', function () {
      const [$updateMeta] = compilePipeline([
        { $updateMeta: { $unset: { foo: '' } } },
      ]);

      it('updates meta', function () {
        const o = $updateMeta({ meta: { foo: 'fooval' } });
        expect(o.meta.foo).to.not.exist;
      });
    });

    describe('{ $rename ... }', function () {
      const [$updateMeta] = compilePipeline([
        { $updateMeta: { $rename: { foo: 'bar' } } },
      ]);

      it('updates meta', function () {
        const o = $updateMeta({ meta: { foo: 'fooval' } });
        expect(o.meta.bar).to.eql('fooval');
      });
    });

    describe('{ ... { $mustache ... } }', function () {
      const [$updateMeta] = compilePipeline([
        {
          $updateMeta: {
            $set: { bar: { $mustache: 'x-{{{meta.foo}}}-y' } },
          },
        },
      ]);

      it('interpolates', function () {
        const o = $updateMeta({ meta: { foo: 'fooval' } });
        expect(o.meta.bar).to.eql('x-fooval-y');
      });
    });

    describe('{ ... { $splitField ... } }', function () {
      const specs = [
        {
          name: 'split comma trim',
          orig: { foo: ' a, b ' },
          args: { field: 'meta.foo', separator: ',', trim: true },
          result: ['a', 'b'],
        },
        {
          name: 'split comma notrim',
          orig: { foo: ' a, b ' },
          args: { field: 'meta.foo', separator: ',', trim: false },
          result: [' a', ' b '],
        },
        {
          name: 'split colon',
          orig: { foo: 'a:b' },
          args: { field: 'meta.foo', separator: ':' },
          result: ['a', 'b'],
        },
        {
          name: 'keep array',
          orig: { foo: ['a, b'] },
          args: { field: 'meta.foo', separator: ',' },
          result: ['a, b'],
        },
        {
          name: 'keep object',
          orig: { foo: { a: 1 } },
          args: { field: 'meta.foo', separator: ',' },
          result: { a: 1 },
        },
        {
          name: 'make array from string',
          orig: { foo: 'a' },
          args: { field: 'meta.foo', separator: ',' },
          result: ['a'],
        },
        {
          name: 'make array from number',
          orig: { foo: 1 },
          args: { field: 'meta.foo', separator: ',' },
          result: [1],
        },
      ];

      for (const s of specs) {
        it(s.name, function () {
          const [op] = compilePipeline([
            { $updateMeta: { $set: { bar: { $splitField: s.args } } } },
          ]);
          const o = op({ meta: s.orig });
          expect(o.meta.bar).to.eql(s.result);
        });
      }
    });
  });
});
