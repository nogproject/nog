/* eslint-disable react/forbid-prop-types */
/* eslint-disable jsx-a11y/anchor-is-valid */

import { _ } from 'meteor/underscore';
import React from 'react';
import PropTypes from 'prop-types';
import Autosuggest from 'react-autosuggest';
import { withTracker } from 'meteor/react-meteor-data';

function Trackee({
  className,
  id,
  placeholder,
  value,
  disabled,
  onChange,
  onKeyPress,
  suggestions,
  onSuggestionsFetchRequested,
  onSuggestionsClearRequested,
}) {
  function getSuggestionValue(sug) {
    return sug.symbol;
  }

  function renderSuggestion(sug) {
    const {
      symbol, names, description, examples,
    } = sug;
    return (
      <a key={symbol}>
        <strong>{symbol}</strong>
        <small> &#8226; {names.join(' | ')}</small>
        <br />
        <span className="text-muted">{description}</span>
        <br />
        <small className="text-muted">Examples: {examples.join(', ')}.</small>
      </a>
    );
  }

  const inputProps = {
    id,
    placeholder,
    value,
    disabled,
    onChange,
    onKeyPress,
  };

  const theme = {
    container: className,
    input: 'form-control',
    inputOpen: '',
    inputFocused: '',
    suggestionsContainer: 'dropdown',
    suggestionsContainerOpen: 'open',
    suggestionsList: 'dropdown-menu show',
    suggestion: '',
    suggestionFirst: '',
    suggestionHighlighted: 'active',
    sectionContainer: '',
    sectionContainerFirst: '',
    sectionTitle: '',
  };

  return (
    <Autosuggest
      suggestions={suggestions}
      onSuggestionsFetchRequested={onSuggestionsFetchRequested}
      onSuggestionsClearRequested={onSuggestionsClearRequested}
      getSuggestionValue={getSuggestionValue}
      renderSuggestion={renderSuggestion}
      inputProps={inputProps}
      theme={theme}
    />
  );
}

Trackee.propTypes = {
  className: PropTypes.string.isRequired,
  id: PropTypes.string.isRequired,
  placeholder: PropTypes.string.isRequired,
  value: PropTypes.string.isRequired,
  disabled: PropTypes.bool.isRequired,
  onChange: PropTypes.func.isRequired,
  onKeyPress: PropTypes.func.isRequired,
  suggestions: PropTypes.array.isRequired,
  onSuggestionsFetchRequested: PropTypes.func.isRequired,
  onSuggestionsClearRequested: PropTypes.func.isRequired,
};

function callFetch(nogSuggest, { sugnss, needle }) {
  const {
    mdProperties,
    callFetchMetadataProperties,
  } = nogSuggest;

  mdProperties.removeExpired();
  callFetchMetadataProperties({ sugnss, needle }, (err, res) => {
    if (err) {
      return;
    }
    mdProperties.insertWithExpiry(res);
  });
}

// `callFetchThrottled` must be defined once outside the `Tracker` closure,
// because it has the throttling state.
const callFetchThrottled = _.throttle(callFetch, 250);

// From
// <https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions>.
function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function Tracker(props) {
  const {
    needle,
    onChangeNeedle,
    nogSuggest,
    sugnss,
    ...childProps
  } = props;

  // `maxSuggestionsCount` limits the number of suggestions.
  const maxSuggestionsCount = 20;

  function doFetch({ value }) {
    callFetchThrottled(nogSuggest, { sugnss, needle: value });
    onChangeNeedle(value);
  }

  function doClear() {
    onChangeNeedle('');
  }

  let suggestions = [];
  if (needle.length >= 2) {
    const sel = {
      sugnss: { $in: sugnss },
      tokens: { $regex: new RegExp(`^${escapeRegExp(needle.toLowerCase())}`) },
    };
    const sort = { symbol: 1 };
    const limit = maxSuggestionsCount;
    suggestions = nogSuggest.mdProperties.find(sel, { sort, limit }).fetch();
    // XXX Maybe dedup suggestions.  Duplicates may result from the same
    // property symbol in multiple namespaces.
  }

  return {
    ...childProps,
    suggestions,
    onSuggestionsFetchRequested: doFetch,
    onSuggestionsClearRequested: doClear,
  };
}

const Tracked = withTracker(Tracker)(Trackee);

class AutosuggestProperty extends React.Component {
  constructor() {
    super();
    this.state = {
      needle: '',
    };
    this.doChangeNeedle = this.doChangeNeedle.bind(this);
  }

  doChangeNeedle(val) {
    this.setState({
      needle: val,
    });
  }

  render() {
    const {
      props,
      doChangeNeedle,
    } = this;
    const {
      needle,
    } = this.state;
    return (
      <Tracked
        {...props}
        needle={needle}
        onChangeNeedle={doChangeNeedle}
      />
    );
  }
}

AutosuggestProperty.propTypes = {
  className: PropTypes.string.isRequired,
  id: PropTypes.string.isRequired,
  placeholder: PropTypes.string.isRequired,
  value: PropTypes.string.isRequired,
  disabled: PropTypes.bool.isRequired,
  onChange: PropTypes.func.isRequired,
  onKeyPress: PropTypes.func,
  nogSuggest: PropTypes.shape({
    mdProperties: PropTypes.object.isRequired,
    callFetchMetadataProperties: PropTypes.func.isRequired,
  }).isRequired,
  sugnss: PropTypes.arrayOf(PropTypes.string).isRequired,
};

AutosuggestProperty.defaultProps = {
  onKeyPress: () => undefined,
};

export {
  AutosuggestProperty,
};
