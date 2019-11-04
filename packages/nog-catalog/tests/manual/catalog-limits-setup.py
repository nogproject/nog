#!/usr/bin/env python3
# vim: sw=4

import nog

# The `POST_BUFFER_SIZE` is lowered to avoid timeouts when sending many small
# objects.
nog.POST_BUFFER_SIZE = 100000

# Create `nRepos` repos in nog, each filled with `nTreesPerRepo` trees.
# Each tree contains `nEntriesPerTree` entries, with metadata.
# See the README on how to create a catalog from this data.
repoBaseName = 'testCatalogUpdateLimits'
nRepos = 20
nTreesPerRepo = 10
nEntriesPerTree = 50


def main():
    for r in range(nRepos):
        repoName = '{0}{1}'.format(repoBaseName, r)
        print('Creating repo {0}'.format(repoName))
        try:
            repo = nog.createRepo(repoName)
        except RuntimeError:
            repo = nog.openRepo(repoName)

        master = repo.getMaster()
        root = nog.Tree()
        root.name = 'root'
        root.meta['files'] = {}

        for t in range(nTreesPerRepo):
            treeName = 'Tree{0}'.format(t)
            tree = nog.Tree()
            tree.name = treeName

            for i in range(nEntriesPerTree):
                entry = nog.Object()
                entry.name = 'entry-{}'.format(i)
                entry.meta['entryNum'] = i
                entry.meta['entryName'] = entry.name
                entry.meta['repoName'] = repoName
                entry.meta['treeName'] = treeName
                tree.append(entry)

            root.append(tree)

        repo.commitTree(subject='images', tree=root, parent=master.sha1)

main()
