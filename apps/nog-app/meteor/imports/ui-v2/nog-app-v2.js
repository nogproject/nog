import { check, Match } from 'meteor/check';
import { mount } from 'react-mounter';
import { LayoutContainerV2 } from './layout-v2.jsx';
import {
  FsoDocsGate,
  FsoListingGateContainer,
  FsoRepoGateContainer,
  FsoRootListGateContainer,
  FsoUntrackedListGateContainer,
} from 'meteor/nog-fso-ui';
import {
  NogHomeContainer,
  FsoCatalogContainer,
} from 'meteor/nog-home-v2';

// `initRouterV2()` configures routing to the React `LayoutContainerV2`.
// `globals.router` is the `FlowRouter`.  `globals` is passed through
// `LayoutContainerV2` as props to the per-route main React components.
function initRouterV2(globals) {
  check(globals, {
    user: Function, // Bound Meteor.user().
    router: Match.Any, // Global FlowRouter.
    nogFso: Match.Any, // Global NogFso=createFsoModuleClient().
    nogSuggest: Match.Any, // Global `NogSuggest=createSuggestModuleClient()`.
    nogCatalog: Match.Any, // Global NogCatalog.
    nogHome: Match.Any, // Global NogHome.
    optShowVersions: Boolean, // Meteor.settings.public.optShowVersions.
    versions: Object, // Meteor.settings.public.versions.
  });
  const { router } = globals;
  // Parameterize route names, so that all links point to V2 routes.  V1 Blaze
  // rendering must not be mix with V2 React rendering, because the other
  // layout would not be updated when navigating between V1 and V2.
  const routes = {
    fsoCatalog: 'fsoCatalogV2',
    fsoListing: 'fsoListingV2',
    fsoRepo: 'fsoRepoV2',
    fsoRootList: 'fsoRootListV2',
    fsoUntrackedList: 'fsoUntrackedListV2',
    fsoDocs: 'fsoDocsV2',
    home: 'homeV2',
  };
  const props = { routes, ...globals };

  router.route('/v2/fso/repos/:repoName+', {
    name: 'fsoRepoV2',
    action() {
      mount(LayoutContainerV2, {
        main: FsoRepoGateContainer,
        ...props,
      });
    },
  });

  router.route('/v2/fso/catalogs/:repoPath+', {
    name: 'fsoCatalogV2',
    action() {
      mount(LayoutContainerV2, {
        main: FsoCatalogContainer,
        ...props,
      });
    },
  });

  router.route('/v2/fso/ls/:path*', {
    name: 'fsoListingV2',
    action() {
      mount(LayoutContainerV2, {
        main: FsoListingGateContainer,
        ...props,
      });
    },
  });

  router.route('/v2/fso/untracked/reg:/:registry/root:/:globalRoot+', {
    name: 'fsoUntrackedListV2',
    action() {
      mount(LayoutContainerV2, {
        main: FsoUntrackedListGateContainer,
        ...props,
      });
    },
  });

  router.route('/v2/fso/untracked/:prefix*', {
    name: 'fsoRootListV2',
    action() {
      mount(LayoutContainerV2, {
        main: FsoRootListGateContainer,
        ...props,
      });
    },
  });

  router.route('/v2/fso/docs/repo:/:repoPath+/file:/:treePath*', {
    name: 'fsoDocsV2',
    action() {
      mount(LayoutContainerV2, {
        main: FsoDocsGate,
        ...props,
      });
    },
  });

  router.route('/v2', {
    name: 'homeV2',
    action() {
      mount(LayoutContainerV2, {
        main: NogHomeContainer,
        ...props,
      });
    },
  });
}

export {
  initRouterV2,
};
