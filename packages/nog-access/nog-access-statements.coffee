NogAccessTest.statements = share.statements = [

  {
    principal: 'role:users'
    action: 'nog-blob/upload'
    effect: 'allow'
  }
  {
    principal: 'role:users'
    action: 'nog-blob/upload'
    effect: (opts) ->
      if not NogAccess.config.uploadSizeLimit
        'ignore'
      else if NogAccess.config.uploadSizeLimit is 0
        'ignore'
      else if not opts?.size?
        'ignore'
      else if opts.size <= NogAccess.config.uploadSizeLimit
        'allow'
      else
        {
          effect: 'deny'
          reason: "
              Upload is larger than the size limit of
              #{NogAccess.config.uploadSizeLimit} Bytes.
            "
        }
  }
  {
    principal: 'role:users'
    action: 'nog-blob/download'
    effect: 'allow'
  }

  # Allow non-guest users to manage their own api keys.
  {
    principal: 'guests'
    action: 'nog-auth/apiKey'
    effect: 'deny'
  }
  {
    principal: /^username:/
    action: 'nog-auth/apiKey'
    effect: (opts) ->
      unless opts.user? and opts.keyOwnerId?
        # This should never happen.  Be defensive and 'deny'.
        return 'deny'
      if opts.user._id == opts.keyOwnerId
        'allow'
      else
        'ignore'
  }

  # Admins as well as vde and spr may change the user roles.
  # `mayModifyUserRoles` is kept only for compatibiliy with the `gui-admin`
  # app.  It can be removed when `gui-admin` is removed.
  {
    principal: 'role:admins'
    action: 'mayModifyUserRoles'
    effect: 'allow'
  }
  {
    principal: 'username:sprohaska'
    action: 'mayModifyUserRoles'
    effect: 'allow'
  }
  {
    principal: 'username:vincentdercksen'
    action: 'mayModifyUserRoles'
    effect: 'allow'
  }

  # Admins may change user roles.  To bootstrap the process, allow vde and spr
  # explicitly to change roles.
  {
    principal: 'role:admins'
    action: 'accounts/modifyRoles'
    effect: 'allow'
  }
  {
    principal: 'username:sprohaska'
    action: 'accounts/modifyRoles'
    effect: 'allow'
  }
  {
    principal: 'username:vincentdercksen'
    action: 'accounts/modifyRoles'
    effect: 'allow'
  }
  {
    principal: 'role:admins'
    action: 'accounts/adminView'
    effect: 'allow'
  }
  {
    principal: 'username:sprohaska'
    action: 'accounts/adminView'
    effect: 'allow'
  }
  {
    principal: 'username:vincentdercksen'
    action: 'accounts/adminView'
    effect: 'allow'
  }

  # Admins may delete users.
  {
    principal: 'role:admins'
    action: 'accounts/delete'
    effect: 'allow'
  }

  # Admins may run db checks.
  {
    principal: 'role:admins'
    action: 'nog-ops/dbck'
    effect: 'allow'
  }

  # Placeholder actions to support toggles 'isUser', ....  They should perhaps
  # be removed when we have defined specific actions for everything that we
  # care about.
  {
    principal: 'role:users'
    action: 'isUser'
    effect: 'allow'
  }
  {
    principal: 'role:admins'
    action: 'isAdmin'
    effect: 'allow'
  }
  {
    principal: 'guests'
    action: 'isGuest'
    effect: 'allow'
  }

  # Users may get any content.
  #
  # Circle sharing is controlled by an additional statement that is inserted
  # when sharing is activated (see `nog-sharing.coffee`).
  {
    principal: 'role:users'
    action: 'nog-content/get'
    effect: 'allow'
  }

  # Users may fork repos.  `nog-content/fork-repo` only means that they may
  # fork in general.  Forking a specific repo also requires `nog-content/get`
  # on the forked repo and `nog-content/create-repo` on the new repo.
  {
    principal: 'role:users'
    action: 'nog-content/fork-repo'
    effect: 'allow'
  }

  # Allow non-guest owners to create repos.
  {
    principal: 'guests'
    action: 'nog-content/create-repo'
    effect: 'deny'
  }
  {
    principal: /// ^ username : [^:]+ $ ///
    action: 'nog-content/create-repo'
    effect: (opts) ->
      userName = opts.principal.split(':')[1]
      if userName is opts.ownerName
        'allow'
      else
        'ignore'
  }

  # Allow non-guest owners to create repo content.
  {
    principal: 'guests'
    action: 'nog-content/modify'
    effect: 'deny'
  }
  {
    principal: /// ^ username : [^:]+ $ ///
    action: 'nog-content/modify'
    effect: (opts) ->
      userName = opts.principal.split(':')[1]
      if userName is opts.ownerName
        'allow'
      else
        'ignore'
  }

  # Allow non-guest owners to delete repos.
  {
    principal: 'guests'
    action: 'nog-content/delete-repo'
    effect: 'deny'
  }
  {
    principal: /// ^ username : [^:]+ $ ///
    action: 'nog-content/delete-repo'
    effect: (opts) ->
      userName = opts.principal.split(':')[1]
      if userName is opts.ownerName
        'allow'
      else
        'ignore'
  }

  # Allow non-guest owners to rename repos.
  {
    principal: 'guests'
    action: 'nog-content/rename-repo'
    effect: 'deny'
  }
  {
    principal: /// ^ username : [^:]+ $ ///
    action: 'nog-content/rename-repo'
    effect: (opts) ->
      username = opts.principal.split(':')[1]
      if (username == opts.old.ownerName and
          username == opts.new.ownerName)
        'allow'
      else
        'ignore'
  }

  # Allow owners to call configureCatalog() and updateCatalog().
  {
    principal: /// ^ username : [^:]+ $ ///
    action: 'nog-catalog/configure'
    effect: (opts) ->
      userName = opts.principal.split(':')[1]
      if userName is opts.ownerName
        'allow'
      else
        'ignore'
  }
  {
    principal: /// ^ username : [^:]+ $ ///
    action: 'nog-catalog/update'
    effect: (opts) ->
      userName = opts.principal.split(':')[1]
      if userName is opts.ownerName
        'allow'
      else
        'ignore'
  }

  # Statements for experimental `nog-sync` package.
  {
    principal: 'role:nogsyncbots'
    action: 'nog-sync/get'
    effect: 'allow'
  }
  {
    principal: 'role:noglocalsyncbots'
    action: 'nog-sync/create'
    effect: 'allow'
  }
  {
    principal: 'role:noglocalsyncbots'
    action: 'nog-sync/modify'
    effect: 'allow'
  }

]
