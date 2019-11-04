// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^meteor/",
          "uuid/v5",
        ],
      },
    ],
  },
};
