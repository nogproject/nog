# CPU profiling

```bash
meteor add nog-perf
```

In the Browser console, start gathering a CPU profile; here for 10 seconds:

```
NogPerf.profileCpu(10)
```

Wait for completion.  Load the profile in Chrome Developer Tools / tab
Profiles.

You should finally remove the package to avoid deploying dead code to
production:

```bash
meteor remove nog-perf
```
