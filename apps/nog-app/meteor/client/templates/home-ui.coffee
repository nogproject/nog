# Copy helpers from Template._loginButtonsLoggedOut.helpers.
# They are needed for the loginBox to work safely.
# This ensures the same behavior as the original template
# by checking for existing account services and their
# configuration.
Template.loginBox.helpers
  dropdown: -> Accounts._loginButtons.dropdown()

  services: -> Accounts._loginButtons.getLoginServices()

  configurationLoaded: -> Accounts.loginServicesConfigured()
