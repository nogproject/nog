NogPerf.profileCpu = (duration_s, outpath) ->

  unless NogPerf.isProfileCpuEnabled
    console.error '[nog-perf] CPU profiling disabled.'
    return

  outpath ?= '/tmp/nog.cpuprofile'
  Meteor.call 'NogPerf.profileCpu', duration_s, outpath, (err, res) ->
    if err
      console.error err
      return
    console.log res

  console.log "
    [nog-perf] start CPU profiling for #{duration_s} seconds.  See server
    console for details.
  "
