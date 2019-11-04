import React from 'react';
const { Fragment } = React;

function PageNotFound() {
  return (
    <Fragment>
      <h1>Page Not Found</h1>
      <p>
        Make sure the address is correct and the page has not moved.
      </p>
    </Fragment>
  );
}

export {
  PageNotFound,
};
