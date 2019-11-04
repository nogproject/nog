#!/usr/bin/env python3
# vim: sw=4

import nog

# The `POST_BUFFER_SIZE` is lowered to avoid timeouts when sending many small
# objects.
nog.POST_BUFFER_SIZE = 100000

# `repoName` is the testing repository name.
repoName = 'test-workspace-loading-performance'

# `nImages` controls the number of images in the datalist and results subtrees.
nImages = 30

# `nResults` control the number of results and the number of sub-results.  The
# overall number of sub-results is `nResults^2`, without repetition.
nResults = 20

# `nRepeatResults` controls how many times a sub-result is repeated.  The
# publication skips identical trees that have been sent before.  The client
# code has problems handling a large number of results.  So don't increase this
# number too much without optimizing the client first.
nRepeatResults = 3


def main():
    try:
        repo = nog.createRepo(repoName)
    except RuntimeError:
        repo = nog.openRepo(repoName)

    master = repo.getMaster()

    root = nog.Tree()
    root.name = 'root'
    root.meta['workspace'] = {}

    datalist = nog.Tree()
    datalist.name = 'datalist'
    datalist.meta['datalist'] = {}
    for i in range(nImages):
        img = nog.Object()
        img.name = 'img-{}.png'.format(i)
        img.blob = '/tmp/test.png'
        datalist.append(img)
    root.append(datalist)

    programs = nog.Tree()
    programs.name = 'programs'
    programs.meta['programs'] = {}
    root.append(programs)

    jobs = nog.Tree()
    jobs.name = 'jobs'
    jobs.meta['jobs'] = {}
    root.append(jobs)

    # The result tree can be organized as:
    #
    # ```
    # /results/result-0/report.md
    # /results/result-1/report.md
    # ...
    # ```
    #
    # or as:
    #
    # ```
    # /results/subresults-0/subresult-0-0/report.md
    # /results/subresults-0/subresult-0-1/report.md
    # /results/subresults-1/subresult-1-0/report.md
    # ...
    # ```
    #
    # Simulate both structures in a single tree.

    results = nog.Tree()
    results.name = 'results'
    results.meta['results'] = {}
    root.append(results)
    for i in range(nResults):
        results.append(makeResultTree('result-{}'.format(i)))
        subresults = nog.Tree()
        subresults.name = 'subresults-{}'.format(i)
        results.append(subresults)
        for j in range(nResults):
            r = makeResultTree('subresult-{}-{}'.format(i, j))
            for k in range(nRepeatResults):
                subresults.append(r)

    repo.commitTree(subject='images', tree=root, parent=master.sha1)


# `makeResultTree()` mimics a hierarchical result with `report.md` in the root
# and details in a subtree.  This structure allows the publication to stop
# without traversing the details subtree.
def makeResultTree(treeName):
    res = nog.Tree()
    res.name = treeName

    report = nog.Object()
    report.name = 'report.md'
    res.append(report)
    lines = [
        '# {}: {} images'.format(treeName, nImages),
        '',
    ]

    images = nog.Tree()
    images.name = 'images'
    res.append(images)
    for i in range(nImages):
        img = nog.Object()
        name = '{}-img-{}.png'.format(treeName, i)
        img.name = name
        img.blob = '/tmp/test.png'
        images.append(img)
        lines.append('<img src="./images/{}" width="20">'.format(name))

    report.text = '\n'.join(lines)
    return res



main()
