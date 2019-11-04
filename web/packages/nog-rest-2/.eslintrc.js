// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^meteor/",
          "body-parser",
          "connect",
          "path-to-regexp",
          "sinon",
          "sinon-chai",
        ],
      },
    ],
  },
};
