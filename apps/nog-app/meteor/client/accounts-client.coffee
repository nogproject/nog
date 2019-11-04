Accounts.ui.config
  requestPermissions:
    github: ['user:email']
  passwordSignupFields: 'USERNAME_AND_EMAIL'

Oidc.registerClient 'gitimp'
Oidc.registerOidcService 'gitimp'
Oidc.registerClient 'gitzib'
Oidc.registerOidcService 'gitzib'

# Redirect to home after logout.
accountsUIBootstrap3.logoutCallback = (err) ->
  if err?
    console.log 'Error while logging out:', err
  FlowRouter.go '/'


# Re-render the current route after a login.
Accounts.onLogin ->
  currentRoute = FlowRouter.current()
  if currentRoute?
    FlowRouter.go currentRoute.path
  else
    FlowRouter.go '/'


# Overwrite the `ian:accounts-ui-bootstrap-3` helper, since the heuristic does
# not work reliably.  Instead, use the explicit `accountType`.
Template._loginButtonsLoggedInDropdownActions.helpers
  allowChangingPassword: -> Meteor.user().accountType is 'password'
