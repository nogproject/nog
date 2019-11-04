// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^meteor/",
          "ldapjs",
        ],
      },
    ],
    "no-underscore-dangle": [
      "error",
      {
        "allow": [
        ],
      },
    ],
    "new-cap": [
      "error",
      {
        "newIsCapExceptions": ["future"],
      },
    ]
  },
};
