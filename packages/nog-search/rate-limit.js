import { Meteor } from 'meteor/meteor';


// `createRateLimiter({ name, maxOps, intervalMs })` returns a `rateLimiter`
// that can be used to throttle code paths by calling `rateLimiter.op()`.  If
// the `maxOps` budget has been consumed for the current time slice of length
// `intervalMs`, the fiber pauses in `op()` until the start of the next time
// slice plus some jitter.  `rateLimiter.op(weight)` can be used to penalize
// operations with a larger `weight`.
//
// `name` is used when reporting that operations for a time slice have been
// limited.  Each time slice will be reported at most once.

function createRateLimiter({ name, maxOps, intervalMs = 1000 }) {
  return {
    name,
    maxOps,
    intervalMs,
    slice: 0,
    count: 0,
    reportedSlice: 0,

    op(weight = 1) {
      for (;;) {
        const nowMs = Date.now();  // Unix ms time.
        const slice = Math.floor(nowMs / this.intervalMs);

        if (slice > this.slice) {
          this.slice = slice;
          this.count = 0;
        }

        if (this.count < this.maxOps) {
          this.count += weight;
          return;
        }

        if (slice > this.reportedSlice) {
          this.reportedSlice = slice;
          const inSliceMs = nowMs - slice * this.intervalMs;
          console.log(
            `[rate-limit] Pausing ${this.name} after ${inSliceMs} ms ` +
            `of time slice ${slice}.`
          );
        }

        const untilMs = (slice + 1 + 0.2 * Math.random()) * this.intervalMs;
        const pauseMs = Math.ceil(untilMs - nowMs);
        Meteor._sleepForMs(pauseMs);
      }
    },
  };
}


export { createRateLimiter };
