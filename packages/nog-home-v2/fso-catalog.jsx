import React from 'react';
import Blaze from 'meteor/gadicc:blaze-react-component';

// `FsoCatalogContainer` wraps a copy of the Blaze catalog UI that has been
// tailored for FSO.  It was the fastest way to get a proof of concept.  We
// will later reimplement the UI in React and then start a separate package
// `nog-catalog-fso-ui`.
function FsoCatalogContainer(props) {
  return (
    <Blaze template="fsoCatalogDiscoverGate" {...props} />
  );
}

export {
  FsoCatalogContainer,
};
