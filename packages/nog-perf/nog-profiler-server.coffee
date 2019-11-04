unless NogPerf.isProfileCpuEnabled
  return


# Inspired by `meteorhacks:kadira-profiler`.
# <https://github.com/meteorhacks/kadira-profiler/blob/master/lib/server.js>.

{writeFileSync} = Npm.require 'fs'


Meteor.methods
  'NogPerf.profileCpu': (duration_s, outpath) ->
    @unblock()
    console.log "[nog-perf] start CPU profiling for #{duration_s} seconds."
    profile = getCpuProfile 'nog', duration_s
    writeFileSync outpath, JSON.stringify(profile)
    msg = "
      [nog-perf] end CPU profiling; saved to `#{outpath}`.  You can load and
      view the profile in the Chrome developer tools tab `Profiles`.
    "
    console.log msg
    return msg


getCpuProfile = Meteor.wrapAsync (name, duration_s, cb) ->
  v8prof = Npm.require 'v8-profiler'
  v8prof.startProfiling name
  finish = ->
    profile = v8prof.stopProfiling name
    cb null, profile
  setTimeout finish, duration_s * 1000
