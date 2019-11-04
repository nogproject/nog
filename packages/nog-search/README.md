# Package `nog-search`

`nog-search` implements a search over the content of repos.  It creates and
continuously updates an index collection `NogContent.contentIndex` of repos and
their entries with documents that contain searchable information such as repo
and owner name, entry path and metadata.  The search is based on the
'EasySearch' package, whose index and engine are configured in
`NogContent.searchContent`.  The provided components of 'EasySearch', e.g. the
input form, are not used in order to enable an extended functionality and
customized design of the search UI in nog.

## Re-usable input form

`nog-search` contains a re-usable input-form template `nogSearchInputForm` that
provides additional functionalities such as resolving and applying the user's
search aliases.  It can be re-used to search and filter in other UIs, e.g., in
catalog view.

To integrate `nogSearchInputForm` into another UI, the template must be
configured via an helper that contains the following fields:

- `inputFormLabel`: label displayed above the input form.
- [optional] `updateOnEnter: true`: updates the search string only on
  pressing 'Enter'.
- `onUpdateInput`: callback that passes the input string to the parent
  template.

```javascript
inputFormParams() {
const tpl = Template.instance();
return {
  inputFormLabel: 'Filter',
  updateOnEnter: true,
  onUpdateInput(str) {
    tpl.inputString.set(str);
  },
};
```

The template can then be inserted into the UI:

```html
{{> nogSearchInputForm params=inputFormParams}}
```
