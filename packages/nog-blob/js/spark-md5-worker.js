var reader = new FileReaderSync();
var chunkSize = 2 * 1024 * 1024;

self.onmessage = function (event) {
    var blob = event.data.blob;
    var spark = new SparkMD5.ArrayBuffer();
    var start, end;
    try {
        for (start = 0; start < blob.size; start += chunkSize) {
            end = Math.min(start + chunkSize, blob.size);
            spark.append(reader.readAsArrayBuffer(blob.slice(start, end)));
        }
        self.postMessage({
            event: 'success',
            hash: spark.end(),
        });
    } catch (e) {
        self.postMessage({
            event: 'error',
            error: e.name,
        });
    }
};
