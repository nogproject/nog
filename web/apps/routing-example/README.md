# App `routing-example`

The app `routing-example` demonstrates how we use React Router for dynamic
routing whit split and nested page components.

In new apps, we will use React Router only.  We import the router module
directly where ever it is needed, we do not inject methods to child components
in order to stay compatible with others like FlowRouter.

The example app splits into two sub-apps, which could be necessary for
different groups of users.  The sub-apps build their view from nested
components.  Hence, we build dynamic routes in the hierarchy, but also need
deep links to other branches of the hierarchy, e.g., to another sub-app.

To dynamically build the routes through the component hierarchy, we inject the
current URL from the Route's props of the parent component via `parentUrl` to
the child component, see also `import/ui/app.jsx`:

```
<Route
  path={routes.vis.path}
  render={({ match }) => (
    <LayoutVis
      parentUrl={match.url}
      toBcpApp={routes.bcp.to}
    />
  )}
/>
```

The route URLs and match patterns will then be defined at the top of the child
component based on the injected `parentUrl` like so:

```
const routes = {
  home: {
    path: `${parentUrl}`,
    to() { return `${parentUrl}`; },
  },
  settings: {
    path: `${parentUrl}/s`,
    to() { return `${parentUrl}/s`; },
  },
};
```

See `import/ui/vis-layout.jsx` for details.

Deep links to another sub-app must be injected at their definition level and
then passed through to the component where it is supposed to be rendered.  We
name such links according to the `to` function of routes, see `toBcpApp` in the
example above, which finally will be used in `import/ui/navbar.jsx`.
