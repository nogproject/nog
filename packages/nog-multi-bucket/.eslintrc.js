// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^aws-sdk$",
          "^meteor/",
        ],
      },
    ],
    "no-underscore-dangle": [
      "error",
      {
        "allowAfterThis": true,
        "allow": [
          "_id",
          "_name",
          "_ensureIndex",
          "_sleepForMs",
        ],
      },
    ],
  },
}
