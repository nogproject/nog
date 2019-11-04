// Check this file's style with:
//
// ```
// ./node_modules/.bin/eslint --ignore-pattern '!.eslintrc.js' .eslintrc.js
// ```
//
module.exports = {
  parser: 'babel-eslint',
  plugins: [
    'meteor',
  ],
  extends: [
    'airbnb',
    'plugin:meteor/recommended',
  ],
  rules: {
    'prefer-arrow-callback': 'off',
    'no-console': 'off',

    'import/no-unresolved': [
      'error',
      {
        ignore: [
          '^meteor/',
        ],
      },
    ],

    'no-underscore-dangle': [
      'error',
      {
        allow: [
          '_id',
          '_sleepForMs',
        ],
      },
    ],

    'new-cap': [
      'error',
      {
        capIsNewExceptions: [
          'Match.Maybe',
          'Match.ObjectIncluding',
          'Match.OneOf',
          'Match.Optional',
          'Match.Where',
        ],
      },
    ],

    quotes: [
      'error',
      'single',
      { allowTemplateLiterals: true },
    ],

    // We have not reach consensus for max-len=79.  You may disable `max-len`
    // on a per-file basis without further discussion.  To do so, put:
    //
    // ```
    // /* eslint-disable max-len */
    // ````
    //
    // at the top of the file.
    'max-len': [
      'error',
      79,
      2,
      { ignoreUrls: true },
    ],

    'no-trailing-spaces': 'error',

    // Allow two spaces before end-of-line comments.
    'no-multi-spaces': [
      'error',
      { ignoreEOLComments: true },
    ],

    // Allow function calls like:
    //
    // ```
    // callFunc({
    //  a, b, c,
    //  e, f,
    // });
    // ```
    //
    'object-property-newline': 'off',

    // Allow line breaks like:
    //
    // ```
    // const m = new Map(
    //   lst.map(({ k, v }) => [k, v]),
    // );
    // ```
    //
    'function-paren-newline': 'off',

    // Allow mixing imports with related statements, like:
    //
    // ```
    // import { NogError } from 'meteor/nog-error';
    // const { nogthrow, createError } = NogError;
    // ```
    //
    'import/newline-after-import': 'off',
    'import/first': 'off',

    // `import/no-extraneous-dependencies` would complain about `import
    // 'meteor/meteor'`.  There is no configuration option to selectively
    // disabled it based on the imported name, <https://goo.gl/8dqruW>.
    'import/no-extraneous-dependencies': 'off',

    // `import/extensions` would complain about `import 'meteor/meteor'`.  We
    // disabled it without further investigation.
    'import/extensions': 'off',

    // Allow named exports for consistency even if a module has only a single
    // export.
    'import/prefer-default-export': 'off',

    // Like `node_modules/eslint-config-airbnb-base/rules/style.js`, but more
    // liberal.
    //
    // Allow `ForOfStatement`.  Refactoring `for..of arr` to `arr.forEach()` is
    // not always straightforward.  `for..of` is more flexible and works, for
    // example, with `Map` and `Set`.
    //
    // Allow `LabeledStatement`.  It feels too dogmatic to disallow them.
    'no-restricted-syntax': [
      'error',
      {
        selector: 'ForInStatement',
        message: (
          'nog/.eslintrc.js: ' +
          '`for .. in` loops iterate over the entire prototype chain, ' +
          'which is virtually never what you want.  ' +
          'Use `for .. of Object.{keys,values,entries}()` instead.'
        ),
      },
      {
        selector: 'WithStatement',
        message: (
          'nog/.eslintrc.js: ' +
          '`with` is disallowed.  ' +
          'It makes code impossible to predict and optimize.'
        ),
      },
    ],

    // The airbnb rule `node_modules/eslint-config-airbnb/rules/react-a11y.js`
    // is incompatible with Bootstrap: it would render the control in bold if
    // we used nesting.  Full rule doc at <https://goo.gl/J3LgLt>.
    'jsx-a11y/label-has-for': [
      'error',
      { components: ['label'], required: 'id' },
    ],

    // Like `node_modules/eslint-config-airbnb-base/rules/style.js`, but also
    // for `ImportDeclaration` and `ExportDeclaration`; see
    // <https://eslint.org/docs/rules/object-curly-newline>.
    'object-curly-newline': ['error', {
      ObjectExpression: { minProperties: 4, multiline: true, consistent: true },
      ObjectPattern: { minProperties: 4, multiline: true, consistent: true },
      ImportDeclaration: { minProperties: 4, multiline: true, consistent: true },
      ExportDeclaration: { minProperties: 4, multiline: true, consistent: true },
    }],
  },
};
