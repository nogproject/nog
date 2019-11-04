import { NogError } from 'meteor/nog-error';
const {
  ERR_BLOB_COMPUTE_MD5,
  createError,
} = NogError;


// `Md5Hasher` computes the MD5 of a blob.  The result is reported via
// callbacks: `onsuccess()` is called with the hash.  `onerror()` is called
// with `Meteor.Error`.

function createMd5Hasher(blob) {
  return {
    onsuccess() {},
    onerror() {},
    start() {
      const worker = new Worker(
        '/packages/nog-blob/' +
        'js/spark-md5.min.da8469403d5f743dd3cb0762146f7b6b67f38867.js'
      );
      worker.onmessage = (e) => {
        worker.terminate();
        if (e.data.event === 'success') {
          this.onsuccess(e.data.hash);
        } else if (e.data.event === 'error') {
          this.onerror(createError(ERR_BLOB_COMPUTE_MD5, {
            cause: e.data.error,
          }));
        }
      };
      worker.postMessage({ blob });
    },
  };
}


export { createMd5Hasher };
