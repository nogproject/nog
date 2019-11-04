// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^meteor/",
          "grpc",
          "jsonwebtoken",
          "moment",
          "node-forge",
          "path-to-regexp",
          "protobufjs",
          "uuid/v5",
        ],
      },
    ],
  },
};
