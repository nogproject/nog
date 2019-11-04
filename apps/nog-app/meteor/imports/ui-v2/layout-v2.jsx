/* eslint-disable react/forbid-prop-types */

import React from 'react';
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import { $ } from 'meteor/jquery';
import { KeyRoute } from 'meteor/nog-fso';
import { Template } from 'meteor/templating';
import { Blaze } from 'meteor/blaze';
// Ensure that `Template._loginButtons` is available.
import 'meteor/ian:accounts-ui-bootstrap-3';

// Use `ref=initTooltip` as the React ref callback,
// <https://reactjs.org/docs/refs-and-the-dom.html>, on an element with
// `data-toggle="tooltip"` to initialize the Bootstrap tooltip, see
// <https://getbootstrap.com/docs/3.3/javascript/#tooltips>.
//
// Use it together with `onClick=hideTooltip` to avoid hanging tooltips.
function initTooltip(el) {
  if (!el) {
    return;
  }
  $(el).tooltip();
}

// Use `onClick=hideTooltip` on a React element to avoid hanging tooltips.
// `tooltip('hide')` must be called on a jQuery object.  It is unclear why
// `ev.currentTarget` works, but `ev.target` does not work.  It is also unclear
// why we need this workaround at all.  See related SO answer:
// <https://stackoverflow.com/a/10569627>
function hideTooltip(ev) {
  $(ev.currentTarget).tooltip('hide');
}

// `NavRightUser` wraps the Blaze login buttons such that they render correctly
// in a Bootstrap navbar.  It is based on the approach in the Meteor React
// tutorial, <https://www.meteor.com/tutorials/react/adding-user-accounts>,
// adapted such that the login buttons element is a direct child of the navbar
// `<ul>`.
//
// We cannot use the Meteor React tutorial approach directly nor can we use
// `<Blaze template=...>` from package `gadicc:blaze-react-component`, because
// both wrap the child element into a span, which would confuse the Bootstrap
// navbar CSS.
//
// Furthermore, we use a React ref function, see
// <https://reactjs.org/docs/refs-and-the-dom.html>.  String refs are
// deprecated, see ESLint rule `react/no-string-refs`.
function NavbarRightUser({ hrefs }) {
  let loginButtonsView = null;

  function manageBlaze(el) {
    if (!el) { // unmount
      Blaze.remove(loginButtonsView);
      return;
    }
    // It is unclear why we must use the template with an underscore, while the
    // template without underscore works in ordinary Blaze.  The underscore
    // version should work fine, though.  See implementation in GitHub
    // `ianmartorell/meteor-accounts-ui-bootstrap-3/login_buttons.html`.
    loginButtonsView = Blaze.render(
      // eslint-disable-next-line no-underscore-dangle
      Template._loginButtons,
      el,
    );
  }

  function liHelp() {
    const href = hrefs.help;
    if (!href) {
      return null;
    }
    return (
      <li>
        <a
          href={href}
          title="Help"
          data-toggle="tooltip"
          data-placement="bottom"
          ref={initTooltip}
          onClick={hideTooltip}
        >
          <span className="glyphicon glyphicon-question-sign" />
        </a>
      </li>
    );
  }

  return (
    <ul ref={manageBlaze} className="nav navbar-nav navbar-right">
      {liHelp()}
    </ul>
  );
}

NavbarRightUser.propTypes = {
  hrefs: PropTypes.shape({
    help: PropTypes.string.isRequired,
  }).isRequired,
};

function NavbarRightUserTracker({ findHrefs }) {
  // The hrefs are resolved here, and not in `LayoutTracker`, so that the main
  // component is isolated from reactive updates here.
  const hrefs = {
    help: findHrefs.help(),
  };
  return {
    hrefs,
  };
}

const NavbarRightUserContainer = (
  withTracker(NavbarRightUserTracker)(NavbarRightUser)
);

NavbarRightUserContainer.propTypes = {
  findHrefs: PropTypes.shape({
    help: PropTypes.func.isRequired,
  }).isRequired,
};

// `NavbarRightAnonymous` is the variant without user.
function NavbarRightAnonymous() {
  return (
    <ul className="nav navbar-nav navbar-right" />
  );
}

// See `../../client/templates/header-ui.jade` for V1 header.
function Header({ hrefs, user, findHrefs }) {
  function navbarRight() {
    if (user) {
      return (
        <NavbarRightUserContainer findHrefs={findHrefs} />
      );
    }
    return (
      <NavbarRightAnonymous />
    );
  }

  return (
    <nav className="navbar navbar-default navbar-static-top">
      <div className="container-fluid">
        <div className="navbar-header">
          <a
            className="navbar-brand"
            href={hrefs.home}
            title="Home screen"
            data-toggle="tooltip"
            data-placement="bottom"
            ref={initTooltip}
            onClick={hideTooltip}
          >
            <span className="glyphicon glyphicon-blackboard" />
          </a>

          <button
            className="navbar-toggle collapsed"
            type="button"
            data-toggle="collapse"
            data-target=".navbar-collapse"
          >
            <span className="sr-only">Toggle navigation</span>
            <span className="icon-bar" />
            <span className="icon-bar" />
            <span className="icon-bar" />
          </button>
        </div>
        <div className="collapse navbar-collapse">
          <ul className="nav navbar-nav">
            <li>
              <a
                href="/"
                title="Click and reload for UI v1"
                data-toggle="tooltip"
                data-placement="bottom"
                ref={initTooltip}
                onClick={hideTooltip}
              >v1
              </a>
            </li>
          </ul>
          {navbarRight()}
        </div>
      </div>
    </nav>
  );
}

Header.propTypes = {
  user: PropTypes.object,
  hrefs: PropTypes.shape({
    home: PropTypes.string.isRequired,
  }).isRequired,
  findHrefs: PropTypes.shape({
    help: PropTypes.func.isRequired,
  }).isRequired,
};

Header.defaultProps = {
  user: null,
};

function Footer({ hrefs, versions }) {
  return [
    <hr key="line" />,
    (
      <div key="text" className="container-fluid">
        <div className="row">
          <div className="col-sm-5">
            <p>
              <small>
                <a href="http://www.zib.de/impressum">Impressum</a>,
                Contact for nog.zib.de: Steffen Prohaska
              </small>
            </p>
          </div>
          <div className="col-sm-2">
            <p className="text-center">
              <small>
                <a href={hrefs.home}>
                  <span className="glyphicon glyphicon-blackboard" />
                </a>
              </small>
            </p>
          </div>
          <div className="col-sm-5">
            <p className="text-right">
              <small>{versions}</small>
            </p>
          </div>
        </div>
      </div>
    ),
  ];
}

Footer.propTypes = {
  hrefs: PropTypes.shape({
    home: PropTypes.string.isRequired,
  }).isRequired,
  versions: PropTypes.string.isRequired,
};

function Layout(props) {
  const {
    router, routes, user,
    optShowVersions, versions,
    findHrefs,
  } = props;

  const hrefs = {
    home: router.path(routes.home),
  };

  function fmtVersions({ app, db }) {
    const vs = [];
    if (app) {
      vs.push(app);
    }
    if (db) {
      vs.push(`db-${db}`);
    }
    if (vs.length === 0) {
      return '';
    }
    return `Version ${vs.join(', ')}`;
  }

  // Assign the dynamic component to a variable to use it in JSX, see
  // <https://reactjs.org/docs/jsx-in-depth.html#choosing-the-type-at-runtime>.
  // Pass the rest of `props` through to the main component.
  const { main: Main, ...mainProps } = props;
  return [
    (
      <Header key="header" user={user} hrefs={hrefs} findHrefs={findHrefs} />
    ),
    (
      <div key="body" className="container-fluid">
        <Main {...mainProps} />
      </div>
    ),
    (
      <Footer
        key="footer"
        hrefs={hrefs}
        versions={optShowVersions ? fmtVersions(versions) : ''}
      />
    ),
  ];
}

Layout.propTypes = {
  main: PropTypes.oneOfType([
    PropTypes.object,
    PropTypes.func,
  ]).isRequired,
  user: PropTypes.object,
  router: PropTypes.object.isRequired,
  routes: PropTypes.shape({
    home: PropTypes.string.isRequired,
    // More unchecked.
  }).isRequired,
  findHrefs: PropTypes.object.isRequired,
  optShowVersions: PropTypes.bool.isRequired,
  versions: PropTypes.object.isRequired,
  nogFso: PropTypes.object.isRequired,
  // More unchecked.
};

function trimSlashes(s) {
  return s.replace(/^\/*/, '').replace(/\/*$/, '');
}

// `LayoutTracker` resolves `user()`.  Children see a static `user`.
function LayoutTracker(props) {
  const {
    router, routes, nogHome,
  } = props;
  const { subscribeHome, homeLinks } = nogHome;

  const user = props.user();
  if (user) {
    subscribeHome();
  }

  // `findHrefs` are function that can be used in child trackers to reactively
  // resolve user-specific hrefs.
  const findHrefs = {
    // `findHrefs.help()` points to the help repo.
    help() {
      const sel = { [KeyRoute]: 'help' };
      const link = homeLinks.findOne(sel);
      if (!link) {
        return '';
      }
      return router.path(routes.fsoDocs, {
        repoPath: trimSlashes(link.path()),
        treePath: 'index.md',
      });
    },
  };

  return { ...props, user, findHrefs };
}

const LayoutContainerV2 = withTracker(LayoutTracker)(Layout);

LayoutContainerV2.propTypes = {
  user: PropTypes.func.isRequired,
  nogHome: PropTypes.object.isRequired,
  router: PropTypes.object.isRequired,
  routes: PropTypes.shape({
    fsoRepo: PropTypes.string.isRequired,
    // More unchecked.
  }).isRequired,
  // More unchecked.
};

export {
  LayoutContainerV2,
};
