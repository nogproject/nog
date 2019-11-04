import { Match, check } from 'meteor/check';
import { Minimongo, LocalCollection } from 'meteor/minimongo';
// eslint-disable-next-line no-underscore-dangle
const minimongoModify = LocalCollection._modify;
import { _ } from 'meteor/underscore';
import Mustache from 'mustache';
import { NogError } from 'meteor/nog-error';
const {
  ERR_PARAM_INVALID,
  nogthrow,
} = NogError;


// `matchCatalogPipelineStep` must be kept in sync with `pipelineFuncMakers`.
//
// Modifiers that are allowed with `$updateMeta` are explicitly listed.  See
// `meteor.git/packages/minimongo/modify.js` `MODIFIERS` for Mongo modifiers
// that are supported by Minimongo and could in principle be allowed.
const matchCatalogPipelineStep = Match.Where((x) => {
  check(x, Match.OneOf(
    { $select: Object },  // Minimongo selector.
    { $addStaticLabels: { labels: Object } },
    {
      $updateName: {
        $select: Match.Optional(Object),
        $set: { $mustache: String },
      },
    },
    {
      $updateMeta: {
        $select: Match.Optional(Object),
        $set: Match.Optional(Object),
        $addToSet: Match.Optional(Object),
        $unset: Match.Optional(Object),
        $rename: Match.Optional(Object),
      },
    },
  ));
  return true;
});


function mapObject(obj, iteratee) {
  const res = {};
  for (const [k, v] of _.pairs(obj)) {
    res[k] = iteratee(v, k);
  }
  return res;
}


function getObjectPath(obj, path) {
  let o = obj;
  for (const key of path.split('.')) {
    o = o[key];
    if (o == null) {
      return undefined;
    }
  }
  return o;
}


// `renderMustache()` returns a copy in which `{ $mustache }` sub-objects are
// replaced with Mustache output, where `content` is the Mustache context.
function renderMustache(obj, content) {
  if (_.isArray(obj)) {
    return obj.map(v => renderMustache(v, content));
  }
  if (!_.isObject(obj)) {
    return obj;
  }

  const fmt = obj.$mustache;
  if (_.isString(fmt)) {
    return Mustache.render(fmt, content);
  }

  return mapObject(obj, v => renderMustache(v, content));
}


// `renderSplitField()` returns a copy in which `{ $splitField }` sub-objects
// are replaced with an array of split values.
//
// `{ $splitField: { field, separator, trim } }` splits the field on the
// separator.  The field will be returned as is if it is already an array.
// Values will be trimmed after split if `trim=true`.
function renderSplitField(obj, content) {
  if (_.isArray(obj)) {
    return obj.map(v => renderSplitField(v, content));
  }

  if (!_.isObject(obj)) {
    return obj;
  }

  const op = obj.$splitField;
  if (_.isObject(op) && op.field) {
    const { field, separator = ',', trim = true } = op;

    let val = getObjectPath(content, field);
    if (_.isArray(val)) {
      return val;
    }
    if (_.isNumber(val)) {
      return [val];
    }
    if (!_.isString(val)) {
      return val;
    }

    val = val.split(separator);
    if (trim) {
      val = val.map(v => v.trim());
    }

    return val;
  }

  return mapObject(obj, v => renderSplitField(v, content));
}


// `pipelineFuncMakers` must be kept in sync with `matchCatalogPipelineStep`.
const pipelineFuncMakers = {
  $select(args) {
    const m = new Minimongo.Matcher(args);
    return function $select(content) {
      if (!m.documentMatches(content).result) {
        return null;
      }
      return content;
    };
  },

  $addStaticLabels(args) {
    const { labels } = args;
    return function $addStaticLabels(content) {
      return {
        ...content,
        meta: { ...content.meta, ...labels },
      };
    };
  },

  $updateName(args) {
    let selector = null;
    if (args.$select) {
      selector = new Minimongo.Matcher(args.$select);
    }
    const fmt = args.$set.$mustache;

    return function $updateName(content) {
      if (selector && !selector.documentMatches(content).result) {
        return content;
      }
      const res = { ...content };
      res.name = Mustache.render(fmt, content);
      return res;
    };
  },

  // The `$updateMeta` modifiers are applied to `content.meta`, so that
  // toplevel fields cannot be modified.
  //
  // See `README.md` for examples.
  //
  // For dynamic rvalues, see `renderMustache()` and `renderSplitField()`.
  $updateMeta(args) {
    let selector = null;
    if (args.$select) {
      selector = new Minimongo.Matcher(args.$select);
    }
    const modifier = _.omit(args, '$select');

    return function $updateMeta(content) {
      if (selector && !selector.documentMatches(content).result) {
        return content;
      }

      let mod = modifier;
      mod = renderMustache(mod, content);
      mod = renderSplitField(mod, content);

      // `minimongoModify()` modifies `meta` in place.
      const meta = { ...content.meta };
      minimongoModify(meta, mod);
      return { ...content, meta };
    };
  },
};


function compilePipeline(pipeline) {
  return pipeline.map((step) => {
    const pairs = _.pairs(step);
    if (pairs.length !== 1) {
      const reason = `Invalid pipeline step \`${JSON.stringify(step)}\`.`;
      nogthrow(ERR_PARAM_INVALID, { reason });
    }

    const [op, args] = pairs[0];

    const makefn = pipelineFuncMakers[op];
    if (!makefn) {
      const reason = `Unknown pipeline function \`${op}\`.`;
      nogthrow(ERR_PARAM_INVALID, { reason });
    }

    return makefn(args);
  });
}


export { matchCatalogPipelineStep, compilePipeline };
