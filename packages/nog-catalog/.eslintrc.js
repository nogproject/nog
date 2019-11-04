// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^meteor/",
          "mustache",
          "sinon",
        ],
      },
    ],
    "no-underscore-dangle": [
      "error",
      {
        "allow": [
          "_dropIndex",
          "_ensureIndex",
          "_id",
          "_name",
          "_sleepForMs",
        ],
      },
    ],
  },
};
