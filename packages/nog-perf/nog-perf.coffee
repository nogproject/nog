NogPerf = {}


if Meteor.absoluteUrl().includes 'http://localhost'
  console.log '[nog-perf] CPU profiling available for app on localhost.'
  NogPerf.isProfileCpuEnabled = true
else
  console.log '[nog-perf] CPU profiling disabled; app not on localhost.'
  NogPerf.isProfileCpuEnabled = false
