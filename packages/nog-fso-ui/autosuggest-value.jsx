/* eslint-disable react/forbid-prop-types */
/* eslint-disable jsx-a11y/anchor-is-valid */

import { _ } from 'meteor/underscore';
import React from 'react';
import PropTypes from 'prop-types';
import Autosuggest from 'react-autosuggest';
import { withTracker } from 'meteor/react-meteor-data';

const TypeidQuantity = 'DTCwugLIUOWhsU9IMbxnpg';

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
      symbol, names, description,
    } = sug;

    if (!description) {
      return (
        <a key={symbol}>
          <strong>{symbol}</strong>
          <small> &#8226; {names.join(' | ')}</small>
        </a>
      );
    }

    return (
      <a key={symbol}>
        <strong>{symbol}</strong>
        <small> &#8226; {names.join(' | ')}</small>
        <br />
        <span className="text-muted">{description}</span>
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

// From
// <https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions>.
function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// `maxSuggestionsCount` limits the number of suggestions.
const maxSuggestionsCount = 20;

const suggesters = {
  TypedItem: {
    suggest(nogSuggest, params, { sugnss, needle }) {
      const { mdItems } = nogSuggest;
      if (needle.length < 2) {
        return [];
      }

      // Present suggestions in `ofType` order.
      let suggestions = [];
      if (needle.length < 2) {
        return suggestions;
      }

      for (const ofType of params.ofType) {
        const sel = {
          sugnss: { $in: sugnss },
          ofType,
          tokens: {
            $regex: new RegExp(`^${escapeRegExp(needle.toLowerCase())}`),
          },
        };
        const sort = { symbol: 1 };
        const limit = maxSuggestionsCount;
        suggestions = suggestions.concat(
          mdItems.find(sel, { sort, limit }).fetch(),
        );
        if (suggestions.length >= maxSuggestionsCount) {
          break;
        }
      }
      return suggestions.slice(0, maxSuggestionsCount);
    },

    fetchItems(nogSuggest, params, { sugnss, needle }) {
      const {
        mdItems,
        callFetchMetadataItems,
      } = nogSuggest;

      callFetchMetadataItems({
        sugnss,
        ids: params.ofType, // Fetch type items.
        needle, ofType: params.ofType, // And items ofType for needle.
      }, (err, res) => {
        if (err) {
          return;
        }
        mdItems.insertWithExpiry(res);
      });
    },
  },

  Quantity: {
    suggest(nogSuggest, params, { sugnss, needle }) {
      const { mdItems } = nogSuggest;

      const tokens = needle.trim().split(' ');
      if (tokens.length > 1) {
        return [];
      }
      const number = tokens[0];

      const sugs = [];

      params.units.forEach((unitId) => {
        const unit = mdItems.findOne({
          sugnss: { $in: sugnss },
          _id: unitId,
        });
        if (!unit) {
          return;
        }
        sugs.push({
          symbol: `${number} ${unit.symbol}`,
          names: unit.names,
          description: '',
        });
      });

      sugs.push({
        symbol: `${number} UNIT`,
        names: ['Quantity'],
        description: 'A quantity should be a number followed by a unit.',
      });

      return sugs;
    },

    fetchItems(nogSuggest, params, { sugnss }) {
      const { mdItems, callFetchMetadataItems } = nogSuggest;

      callFetchMetadataItems({
        sugnss,
        ids: [TypeidQuantity].concat(params.units),
        needle: '', ofType: [],
      }, (err, res) => {
        if (err) {
          return;
        }
        mdItems.insertWithExpiry(res);
      });
    },
  },
};

function callFetch(nogSuggest, { sugnss, propertySymbol, needle }) {
  const {
    mdPropertyTypes,
    mdItems,
    callFetchMetadataPropertyTypes,
  } = nogSuggest;

  mdPropertyTypes.removeExpired();
  mdItems.removeExpired();

  function fetchPropertyType(next) {
    const symbols = [propertySymbol];
    callFetchMetadataPropertyTypes({ sugnss, symbols }, (err, res) => {
      if (err) {
        return;
      }
      mdPropertyTypes.insertWithExpiry(res);
      if (next) {
        next();
      }
    });
  }

  function fetchItems() {
    mdPropertyTypes.find({
      sugnss: { $in: sugnss },
      symbol: propertySymbol,
    }).forEach((datatype) => {
      const suggester = suggesters[datatype.suggest];
      if (!suggester) {
        return;
      }
      suggester.fetchItems(
        nogSuggest, datatype.suggestParams, { sugnss, needle },
      );
    });
  }

  if (mdPropertyTypes.findOne({
    sugnss: { $in: sugnss },
    symbol: propertySymbol,
  })) {
    fetchItems(); // Fetch items using cached datatype.
    fetchPropertyType(); // Refresh datatype.
  } else {
    // Fetch datatype first.
    fetchPropertyType(fetchItems);
  }
}

// `callFetchThrottled` must be defined once outside the `Tracker` closure,
// because it has the throttling state.
const callFetchThrottled = _.throttle(callFetch, 250);

function Tracker(props) {
  const {
    needle,
    onChangeNeedle,
    propertySymbol,
    nogSuggest,
    sugnss,
    ...passProps
  } = props;
  const {
    mdPropertyTypes,
  } = nogSuggest;

  function doFetch({ value }) {
    callFetchThrottled(nogSuggest, {
      sugnss,
      propertySymbol,
      needle: value,
    });
    onChangeNeedle(value);
  }

  function doClear() {
    onChangeNeedle('');
  }

  const childProps = {
    ...passProps,
    onSuggestionsFetchRequested: doFetch,
    onSuggestionsClearRequested: doClear,
  };

  let suggestions = [];
  mdPropertyTypes.find({
    sugnss: { $in: sugnss },
    symbol: propertySymbol,
  }).forEach((datatype) => {
    const suggester = suggesters[datatype.suggest];
    if (!suggester) {
      console.error('Unknown suggest type.');
      return;
    }
    suggestions = suggestions.concat(suggester.suggest(
      nogSuggest, datatype.suggestParams, { sugnss, needle },
    ));
  });
  // XXX Maybe dedup suggestions if they were gathered for multiple datatypes.
  childProps.suggestions = suggestions;

  return childProps;
}

const Tracked = withTracker(Tracker)(Trackee);

class AutosuggestValue extends React.Component {
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

AutosuggestValue.propTypes = {
  className: PropTypes.string.isRequired,
  id: PropTypes.string.isRequired,
  placeholder: PropTypes.string.isRequired,
  value: PropTypes.string.isRequired,
  disabled: PropTypes.bool.isRequired,
  onChange: PropTypes.func.isRequired,
  onKeyPress: PropTypes.func,
  propertySymbol: PropTypes.string.isRequired,
  nogSuggest: PropTypes.shape({
    mdProperties: PropTypes.object.isRequired,
    callFetchMetadataProperties: PropTypes.func.isRequired,
  }).isRequired,
  sugnss: PropTypes.arrayOf(PropTypes.string).isRequired,
};

AutosuggestValue.defaultProps = {
  onKeyPress: () => undefined,
};

export {
  AutosuggestValue,
};
