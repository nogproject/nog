# Test: Workspace performance

Purpose: Verify that workspace loading is fast for larger repos.

## Steps

### Setup

The tests use a testing repo that contains a large workspace.  Usually, do not
create such a repo in a production deployment.  The workspace is large enough
so that you should discover performance problems locally.

To create a testing repo, put a test image at `/tmp/test.png`.  The size of the
image is irrelevant.  Then run:

```
./workspace-performance--setup.py
```

### Verify performance

Try various parts of the UI and verify performance.  You can change the size of
the workspace by modifying the setup script.

### Verify that sub-results are rendered on demand

Browse the results and open sub-results.  Sub-results should render on demand.
You can verify it by opening a subresult with a high second number, like
`subresult-0-29`.  Do the rendering of images start immediately?  If not, it
might be blocked by background rendering of other sub-results.

Reload the workspace.  Wait for it to display.  Then navigate to file view.
File view should load immediately.  A long loading time may indicate that
background rendering of sub-results blocked the method call queue.
