// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^meteor/",
          "underscore",
          "node-forge",
          "jsonwebtoken",
        ],
      },
    ],
  },
};
