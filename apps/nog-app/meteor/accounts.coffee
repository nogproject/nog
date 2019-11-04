if (url = Meteor.settings.public?.ROOT_URL)?
  console.log '[app] Using ROOT_URL from settings.'
  Meteor.absoluteUrl.defaultOptions.rootUrl = url
