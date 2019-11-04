# Test: resolveImgSrc optimization

- Purpose: Reproduce slow rendering of Markdown with many inline images.
  Verify that caching optimization works.

## Steps

### Setup

Create a testing repo that contains a Markdown file with many inline images.
The same image can be used for different paths, because the path lookup is the
bottleneck.

Put a test image at `/tmp/test.png`.  The size of the image is irrelevant.  It
will be rendered only as small squares.  The image should contain color that
can be easily distinguished from gray.

Create the test repo with:

```bash
./resolveimgsrc-optimization--setup.py
```

### Verify performance

Open the test Markdown in Chrome:

```
http://localhost:3000/${NOG_USERNAME}/test-resolveimgsrc-optimization/files/images.md
```

You should see small gray squares that are incrementally replaced by the test
image.

Drop the cache to reproduce the result before the optimization:

```
$ meteor mongo
> db.cache.resolveImgSrc.drop()
```

Reload.  Expected cold cache performance: 200 images, more than 15s wall time,
node 100% CPU, mongod 15% CPU, MacBook Pro 2013, 2.7 GHz Intel Core i7.

Reload again.  Expected hot cache performance: less than 2s wall time.
