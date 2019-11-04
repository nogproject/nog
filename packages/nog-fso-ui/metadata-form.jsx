/* eslint-disable react/forbid-prop-types */

// # Design overview
//
// `MetadataForm` parses the metadata into a list `kvs` of objects `{key,val}`,
// where `val` contains type-dependent details how to display the value:
//
//  - `createString()`, `StringInput`: editable string.
//  - `createStringList()`, `StringListInput`: editable list of strings.
//  - `createOpaque()`, `OpaqueInput`: immutable value of unknown type.
//
// The child components report changes up to `MetadataForm`, which updates the
// `kvs` list.  React then propagates the updates back to the children.

import React from 'react';
import PropTypes from 'prop-types';
import { Match } from 'meteor/check';
import { AutosuggestProperty } from './autosuggest-property.jsx';
import { AutosuggestValue } from './autosuggest-value.jsx';

const KEY_ENTER = 13;

const helpUpdateCatalogs = `
"Update Catalogs" forces an update of the catalogs that include this repo.
Manually triggering updates is usually not necessary.  Catalogs are
automatically updated in the background unless an event gets lost, for example
during a server restart.  Updates may take some time: seconds, if the system is
idle, up to minutes or longer, if there are concurrent changes.
`.replace(/\n/g, ' ').trim();

// Forward decl `createStringList` to use it in `StringInput`.
let createStringList = null;

function StringListInput({
  keyName, val, disabled,
  onChangeValue, onDelete,
  nogSuggest, sugnss,
}) {
  const htmlId = `input-${keyName}`;

  function doChange(ev, { newValue }) {
    onChangeValue({
      key: keyName,
      val: { ...val, staging: newValue },
    });
  }

  function doKeyPress(ev) {
    if (ev.which !== KEY_ENTER) {
      return;
    }
    if (val.staging === '') {
      return;
    }
    onChangeValue({
      key: keyName,
      val: {
        ...val,
        values: val.values.concat([val.staging]),
        staging: '',
      },
    });
  }

  function handleClickDelete(ev) {
    ev.preventDefault();
    onDelete(keyName);
  }

  function htmlItem(item, idx) {
    function ignoreEvent(ev) {
      ev.preventDefault();
    }

    function handleClickDeleteItem(ev) {
      ev.preventDefault();
      onChangeValue({
        key: keyName,
        val: {
          ...val,
          values: val.values.filter((e, i) => i !== idx),
        },
      });
    }

    return (
      <div key={`${idx}`} className="btn-group btn-group-sm">
        <button
          className="btn btn-default"
          onClick={ignoreEvent}
        >
          {item}
        </button>
        <button
          className="btn btn-default"
          onClick={handleClickDeleteItem}
        >
          <span className="glyphicon glyphicon-remove" />
        </button>
      </div>
    );
  }

  const htmlItems = val.values.map(htmlItem);

  return (
    <div className="form-group">
      <label
        className="col-sm-2 control-label"
        htmlFor={htmlId}
      >
        {keyName}
      </label>
      <div className="col-sm-9">
        {htmlItems}
      </div>
      <div className="col-sm-1">
        <button
          className="btn btn-danger btn-sm"
          type="button"
          onClick={handleClickDelete}
        >
          Delete
        </button>
      </div>
      <AutosuggestValue
        className="col-sm-offset-2 col-sm-9"
        id={htmlId}
        placeholder="new item"
        disabled={disabled}
        value={val.staging}
        onChange={doChange}
        onKeyPress={doKeyPress}
        propertySymbol={keyName}
        nogSuggest={nogSuggest}
        sugnss={sugnss}
      />
      <p className="col-sm-offset-2 col-sm-10 help-block">
        String list.
      </p>
    </div>
  );
}

StringListInput.propTypes = {
  keyName: PropTypes.string.isRequired,
  val: PropTypes.object.isRequired,
  disabled: PropTypes.bool.isRequired,
  onChangeValue: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  nogSuggest: PropTypes.object.isRequired,
  sugnss: PropTypes.array.isRequired,
};

function StringInput({
  keyName, val, disabled,
  onChangeValue, onDelete,
  nogSuggest, sugnss,
}) {
  const htmlId = `input-${keyName}`;
  const htmlValue = val.value;

  function doChange(ev, { newValue }) {
    onChangeValue({
      key: keyName,
      val: { ...val, value: newValue },
    });
  }

  function doClickDelete(ev) {
    ev.preventDefault();
    onDelete(keyName);
  }

  function doClickSplit(ev) {
    ev.preventDefault();
    const vals = val.value === '' ? (
      []
    ) : (
      val.value.split(',').map(v => v.trim())
    );
    onChangeValue({
      key: keyName,
      val: createStringList({
        values: vals,
        staging: '',
      }),
    });
  }

  return (
    <div className="form-group">
      <label
        className="col-sm-2 control-label"
        htmlFor={htmlId}
      >
        {keyName}
      </label>
      <AutosuggestValue
        className="col-sm-7"
        id={htmlId}
        placeholder="text, ..."
        disabled={disabled}
        value={htmlValue}
        onChange={doChange}
        propertySymbol={keyName}
        nogSuggest={nogSuggest}
        sugnss={sugnss}
      />
      <div className="col-sm-2">
        <button
          className="btn btn-default"
          type="button"
          onClick={doClickSplit}
        >
          As List
        </button>
      </div>
      <div className="col-sm-1">
        <button
          className="btn btn-danger btn-sm"
          type="button"
          onClick={doClickDelete}
        >
          Delete
        </button>
      </div>
      <p className="col-sm-offset-2 col-sm-10 help-block">
        String value.
      </p>
    </div>
  );
}

StringInput.propTypes = {
  keyName: PropTypes.string.isRequired,
  val: PropTypes.object.isRequired,
  disabled: PropTypes.bool.isRequired,
  onChangeValue: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  nogSuggest: PropTypes.object.isRequired,
  sugnss: PropTypes.array.isRequired,
};

function OpaqueInput({
  keyName, val,
}) {
  const htmlId = `input-${keyName}`;
  const valText = val.json;
  return (
    <div className="form-group">
      <label
        className="col-sm-2 control-label"
        htmlFor={htmlId}
      >
        {keyName}
      </label>
      <div className="col-sm-10">
        <input
          type="text"
          className="form-control"
          id={htmlId}
          placeholder={valText}
          disabled
        />
      </div>
      <p className="col-sm-12 help-block">
        Unknown value type; displaying JSON representation.
      </p>
    </div>
  );
}

OpaqueInput.propTypes = {
  keyName: PropTypes.string.isRequired,
  val: PropTypes.object.isRequired,
};

function StringListView({
  keyName, val,
}) {
  return (
    <div className="row">
      <div className="col-sm-2">
        <strong>{keyName}</strong>
      </div>
      <div className="col-sm-10">
        <p>{val.values.join(', ')}</p>
      </div>
    </div>
  );
}

StringListView.propTypes = {
  keyName: PropTypes.string.isRequired,
  val: PropTypes.object.isRequired,
};

function StringView({
  keyName, val,
}) {
  return (
    <div className="row">
      <div className="col-sm-2">
        <strong>{keyName}</strong>
      </div>
      <div className="col-sm-10">
        <p>{val.value}</p>
      </div>
    </div>
  );
}

StringView.propTypes = {
  keyName: PropTypes.string.isRequired,
  val: PropTypes.object.isRequired,
};

function OpaqueView({
  keyName, val,
}) {
  return (
    <div className="row">
      <div className="col-sm-2">
        <strong>{keyName}</strong>
      </div>
      <div className="col-sm-10">
        <p>{val.json}</p>
      </div>
    </div>
  );
}

OpaqueView.propTypes = {
  keyName: PropTypes.string.isRequired,
  val: PropTypes.object.isRequired,
};

// eslint-disable-next-line no-shadow
createStringList = function createStringList({ values, staging }) {
  return {
    view: StringListView,
    input: StringListInput,
    values,
    staging,
    marshal() {
      return this.values;
    },
  };
};

function createString(value) {
  return {
    view: StringView,
    input: StringInput,
    value,
    marshal() {
      return this.value;
    },
  };
}

function createOpaque(value) {
  let json = null;
  try {
    json = JSON.stringify(value);
  } catch (err) {
    json = 'Failed to encode as JSON.';
  }
  return {
    view: OpaqueView,
    input: OpaqueInput,
    original: value,
    json,
    marshal() {
      return this.original;
    },
  };
}

function unmarshalVal(v) {
  if (Match.test(v, [String])) {
    return createStringList({
      values: v,
      staging: '',
    });
  }
  if (Match.test(v, String)) {
    return createString(v);
  }
  return createOpaque(v);
}

function unmarshalMeta(meta) {
  const lst = [];
  for (const [key, v] of meta.entries()) {
    const val = unmarshalVal(v);
    lst.push({ key, val });
  }
  return lst;
}

function marshalMeta(lst) {
  const meta = new Map();
  for (const kv of lst) {
    meta.set(kv.key, kv.val.marshal());
  }
  return meta;
}

class MetadataForm extends React.Component {
  constructor(props) {
    super(props);
    const {
      receivedMetaCommit,
      receivedMetadata,
      metaIsUpdating,
      metaIsSaving,
    } = props;
    this.state = {
      kvs: unmarshalMeta(receivedMetadata),
      currentMetaCommit: receivedMetaCommit,
      newPropertyName: '',
      isEditing: false,
      hasExternalChanges: false,
      hasCommittedChanges: false,
      editingIsDisabled: metaIsUpdating || metaIsSaving,
      savingIsDisabled: metaIsUpdating || metaIsSaving,
    };
    this.handleClickSave = this.handleClickSave.bind(this);
    this.handleClickCancel = this.handleClickCancel.bind(this);
    this.handleClickUpdateCatalogs = this.handleClickUpdateCatalogs.bind(this);
    this.updateKv = this.updateKv.bind(this);
    this.deleteKv = this.deleteKv.bind(this);
    this.doChangeNewProperty = this.doChangeNewProperty.bind(this);
    this.handleClickAddStatement = this.handleClickAddStatement.bind(this);
    this.handleKeyPressAddStatement = (
      this.handleKeyPressAddStatement.bind(this)
    );
    this.handleToggleEdit = this.handleToggleEdit.bind(this);
    this.handleClickLoadExternalChanges = (
      this.handleClickLoadExternalChanges.bind(this)
    );
  }

  // Future versions of React will remove currently used lifecycle methods and
  // enforce safe props/state usage. Current `props` should be stored to
  // `state`, so that information from previous `props` can be used.  See
  // https://reactjs.org/blog/2018/03/27/update-on-async-rendering.html#updating-state-based-on-props
  componentWillReceiveProps(props) {
    const {
      receivedMetaCommit,
      receivedMetadata,
      committedMetaCommit,
      metaIsUpdating,
      metaIsSaving,
    } = props;
    const {
      isEditing,
      currentMetaCommit,
      hasExternalChanges,
      hasCommittedChanges,
    } = this.state;
    this.setState({
      editingIsDisabled: metaIsUpdating || metaIsSaving,
      savingIsDisabled: metaIsUpdating || metaIsSaving || hasExternalChanges,
    });

    // Concurrent metadata changes can cause different situations and requires
    // decisions which metadata to display.
    // 1 External changes are received and the client is in edit mode: an
    //   error message will be displayed; the modified metadata on the client
    //   cannot be saved;  The latest metadata must be loaded before modifying
    //   again.
    // 2 External changes appear after the client has committed its
    //   changes: the locally modified metadata will be displayed; a warning
    //   about external changes and a load button are displayed; the user can
    //   decide whether to view or to load;  edit mode is disabled until the
    //   user loads the latest metadata.
    // 3 no concurrent changes: the latest metadata will be loaded and
    //   displayed.
    if (hasCommittedChanges) {
      if (committedMetaCommit.date > currentMetaCommit.date) {
        this.setState({ currentMetaCommit: committedMetaCommit });
      }
      if (receivedMetaCommit.date > committedMetaCommit.date) {
        this.setState({
          hasExternalChanges: true,
          editingIsDisabled: true,
        });
      }
    } else if (receivedMetaCommit.date > currentMetaCommit.date) {
      if (isEditing) {
        this.setState({
          hasExternalChanges: true,
        });
      } else {
        this.setState({
          kvs: unmarshalMeta(receivedMetadata),
          currentMetaCommit: receivedMetaCommit,
        });
      }
    }
  }

  // React recommends to use `componentDidUpdate` to call external callbacks:
  // https://reactjs.org/blog/2018/03/27/update-on-async-rendering.html#invoking-external-callbacks
  componentDidUpdate(prevProps, prevState) {
    const { receivedMetaCommit } = this.props;
    const { hasExternalChanges, isEditing } = this.state;
    if (isEditing && hasExternalChanges &&
      hasExternalChanges !== prevState.hasExternalChanges) {
      this.props.onExternalChange({
        title: 'Update conflict',
        message: `Your changes cannot be saved, because user
${receivedMetaCommit.author} concurrently modified the metadata at
${receivedMetaCommit.date}. To resolve the conflict, either open the repo in a
second window and copy your changes to there or click 'Cancel' to discard your
changes.`,
      });
    }
  }

  handleClickSave(ev) {
    ev.preventDefault();
    const { onSubmitSaveMeta, repoId, repoPath } = this.props;
    const meta = marshalMeta(this.state.kvs);
    onSubmitSaveMeta({ repoId, repoPath, meta });
    this.setState({
      isEditing: false,
      hasCommittedChanges: true,
    });
  }

  handleClickCancel(ev) {
    ev.preventDefault();
    const {
      receivedMetaCommit,
      receivedMetadata,
      metaIsUpdating,
      metaIsSaving,
    } = this.props;
    this.setState({
      kvs: unmarshalMeta(receivedMetadata),
      currentMetaCommit: receivedMetaCommit,
      isEditing: false,
      hasExternalChanges: false,
      hasCommittedChanges: false,
      editingIsDisabled: metaIsUpdating || metaIsSaving,
      savingIsDisabled: metaIsUpdating || metaIsSaving,
    });
  }

  handleClickUpdateCatalogs(ev) {
    ev.preventDefault();
    const { onUpdateCatalogs, repoId, repoPath } = this.props;
    onUpdateCatalogs({ repoId, repoPath });
  }

  updateKv({ key, val }) {
    this.setState({
      kvs: this.state.kvs.map((kv) => {
        if (kv.key === key) {
          return { key, val };
        }
        return kv;
      }),
    });
  }

  deleteKv(key) {
    this.setState({
      kvs: this.state.kvs.filter(kv => kv.key !== key),
    });
  }

  doChangeNewProperty(ev, { newValue }) {
    this.setState({
      newPropertyName: newValue,
    });
  }

  handleClickAddStatement(ev) {
    ev.preventDefault();
    this.addStatement();
  }

  handleKeyPressAddStatement(ev) {
    if (ev.which !== KEY_ENTER) {
      return;
    }
    this.addStatement();
  }

  addStatement() {
    const { kvs, newPropertyName: key } = this.state;
    if (!key.match(/^[a-z0-9_]{3,}$/)) {
      console.error('Cannot add property: invalid property name.');
      return;
    }
    if (kvs.find(kv => kv.key === key)) {
      console.error(`Cannot add property: duplicate key \`${key}\`.`);
      return;
    }
    const val = createString('');
    this.setState({
      kvs: kvs.concat([{ key, val }]),
      newPropertyName: '',
    });
  }

  handleToggleEdit() {
    this.setState({
      isEditing: !this.state.isEditing,
    });
  }

  handleClickLoadExternalChanges(ev) {
    ev.preventDefault();
    const {
      receivedMetaCommit,
      receivedMetadata,
      metaIsUpdating,
      metaIsSaving,
    } = this.props;
    this.setState({
      currentMetaCommit: receivedMetaCommit,
      kvs: unmarshalMeta(receivedMetadata),
      hasExternalChanges: false,
      hasCommittedChanges: false,
      editingIsDisabled: metaIsUpdating || metaIsSaving,
      savingIsDisabled: metaIsUpdating || metaIsSaving,
    });
  }

  render() {
    const {
      handleClickSave,
      handleClickCancel,
      handleClickUpdateCatalogs,
      updateKv,
      deleteKv,
      doChangeNewProperty,
      handleClickAddStatement,
      handleKeyPressAddStatement,
      handleToggleEdit,
      handleClickLoadExternalChanges,
      state,
    } = this;
    const {
      nogSuggest,
      sugnss,
      receivedMetaCommit,
      metaIsSaving,
    } = this.props;
    const {
      newPropertyName,
      isEditing,
      savingIsDisabled,
      editingIsDisabled,
      hasCommittedChanges,
      hasExternalChanges,
    } = state;

    function warnExternalChanges() {
      if (hasCommittedChanges && hasExternalChanges && !isEditing) {
        const msg = `
There are newer changes made by ${receivedMetaCommit.author} at
${receivedMetaCommit.date.toString()}.  Reload to view or edit the latest
metadata.`;
        return (
          <div className="alert alert-warning" role="alert">
            <div className="row">
              <div className="col-md-11">
                {msg}
              </div>
              <div className="col-md-1">
                <button
                  className="btn btn-default btn-xs"
                  onClick={handleClickLoadExternalChanges}
                >
                  Load
                </button>
              </div>
            </div>
          </div>
        );
      }
      return null;
    }

    // These properties are displayed at the top in this order.
    const wellKnownKeysOrder = [
      'title',
      'description',
      'keywords',
    ];

    function inputForm() {
      const wellKnown = new Map();
      const inputs = [];
      for (const { key, val } of state.kvs) {
        const Input = val.input;
        if (!Input) {
          continue; // eslint-disable-line no-continue
        }
        const input = (
          <Input
            key={key}
            keyName={key}
            val={val}
            disabled={editingIsDisabled}
            onChangeValue={updateKv}
            onDelete={deleteKv}
            nogSuggest={nogSuggest}
            sugnss={sugnss}
          />
        );
        if (wellKnownKeysOrder.includes(key)) {
          wellKnown.set(key, input);
        } else {
          inputs.push(input);
        }
      }
      inputs.unshift(
        ...wellKnownKeysOrder.map(k => wellKnown.get(k)),
      );

      return (
        <div className="form-horizontal">
          {inputs}
          <div className="form-group">
            <label
              className="col-sm-2"
              htmlFor="__addStatement__"
            >
              <button
                className="btn btn-default"
                type="button"
                onClick={handleClickAddStatement}
              >
                Add field
              </button>
            </label>
            <AutosuggestProperty
              className="col-sm-9"
              id="__addStatement__"
              placeholder="new field name"
              disabled={editingIsDisabled}
              value={newPropertyName}
              onChange={doChangeNewProperty}
              onKeyPress={handleKeyPressAddStatement}
              nogSuggest={nogSuggest}
              sugnss={sugnss}
            />
            <p className="col-sm-offset-2 col-sm-10 help-block">
              To add a metadata field, enter its name and press ENTER or{' '}
              click &quot;Add field&quot;.
            </p>
          </div>
        </div>
      );
    }

    function readonlyView() {
      const wellKnown = new Map();
      const views = [];
      for (const { key, val } of state.kvs) {
        const View = val.view;
        if (!View) {
          continue; // eslint-disable-line no-continue
        }

        if (key === 'title' && View === StringView) {
          wellKnown.set('title', (
            <div key={key} className="row">
              <div className="col-sm-2">
                <h3>{key}</h3>
              </div>
              <div className="col-sm-10">
                <h3>{val.value}</h3>
              </div>
            </div>
          ));
          continue; // eslint-disable-line no-continue
        }

        const view = (
          <View
            key={key}
            keyName={key}
            val={val}
          />
        );
        if (wellKnownKeysOrder.includes(key)) {
          wellKnown.set(key, view);
        } else {
          views.push(view);
        }
      }

      if (wellKnown.size === 0 && views.length === 0) {
        return (
          <p>No metadata.</p>
        );
      }

      views.unshift(
        ...wellKnownKeysOrder.map(k => wellKnown.get(k)),
      );

      return (
        <div className="form-horizontal">
          {views}
        </div>
      );
    }

    return (
      <div className="row">
        <div className="col-md-12">
          <div className="panel panel-default">
            <div className="panel-heading">
              <h3 className="panel-title">
                Metadata
                &nbsp;
                &nbsp;
                &nbsp;
                <small>
                  <input
                    type="checkbox"
                    checked={isEditing}
                    disabled={editingIsDisabled}
                    onChange={handleToggleEdit}
                  /> edit
                  &nbsp;
                  &nbsp;
                  {isEditing || metaIsSaving ? (
                    <span>
                      <button
                        className="btn btn-primary btn-xs"
                        onClick={handleClickSave}
                        disabled={savingIsDisabled}
                      >
                        Save
                      </button>
                      <button
                        className="btn btn-danger btn-xs"
                        onClick={handleClickCancel}
                        disabled={editingIsDisabled}
                      >
                        Cancel
                      </button>
                    </span>
                  ) : (
                    <span>
                      <button
                        className="btn btn-default btn-xs"
                        onClick={handleClickUpdateCatalogs}
                        title={helpUpdateCatalogs}
                      >
                        Update Catalogs
                      </button>
                    </span>
                  )}
                </small>
              </h3>
            </div>
            <div className="panel-body">
              {warnExternalChanges()}
              {isEditing || metaIsSaving ? inputForm() : readonlyView()}
            </div>
          </div>
        </div>
      </div>
    );
  }
}

// The fields of the prop `meta` may be empty or undefined if no metadata exist
// or the published document is not yet complete.  The MetadataForm must work
// anyway to provide the input possibility.  The commitId is just used for
// comparison to check changes and, thus, may be empty.  `author` and
// `commitId` are only dispayed in the warning when an external meta commit is
// received.
MetadataForm.propTypes = {
  repoId: PropTypes.string.isRequired,
  repoPath: PropTypes.string.isRequired,
  onExternalChange: PropTypes.func.isRequired,
  onSubmitSaveMeta: PropTypes.func.isRequired,
  onUpdateCatalogs: PropTypes.func.isRequired,
  metaIsUpdating: PropTypes.bool.isRequired,
  metaIsSaving: PropTypes.bool.isRequired,
  receivedMetaCommit: PropTypes.shape({
    author: PropTypes.string,
    date: PropTypes.object,
    commitId: PropTypes.string,
  }).isRequired,
  receivedMetadata: PropTypes.object.isRequired,
  committedMetaCommit: PropTypes.object.isRequired,
  nogSuggest: PropTypes.object.isRequired,
  sugnss: PropTypes.array.isRequired,
};

export {
  MetadataForm,
};
