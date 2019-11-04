// See toplevel `../../.eslintrc.js` for defaults.

module.exports = {
  rules: {
    "import/no-unresolved": [
      "error",
      {
        "ignore": [
          "^meteor/",
          "highlight.js",
          "markdown-toc",
          "marked",
          "prop-types",
          "react",
          "sanitize-html",
        ],
      },
    ],
  },
};
