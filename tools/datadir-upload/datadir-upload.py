#!/usr/bin/env python3

"""Dataset Uploader.

Usage:
    dataset-uploader.py [--force-overwrite] <dir-to-upload> <repository-name>

This script uploads a directory to a Nog repository.  The entire directory tree
is uploaded.  If the repository does not exist, it is created.  If an entry
with the same name as the directory is already present in the repo, nothing is
uploaded unless the '--force-overwrite' option is used.  In this case, the
original entry is replaced.

"""

from docopt import docopt
import nog
import os

def addContentsOfDirToTree(dirpath, tree):
    for entry in os.listdir(dirpath):
        entrypath = os.path.join(dirpath, entry)
        if os.path.isfile(entrypath):
            obj = nog.Object()
            obj.name = entry
            obj.blob = entrypath
            tree.append(obj)
        elif os.path.isdir(entrypath):
            subtree = nog.Tree()
            subtree.name = entry
            tree.append(subtree)
            addContentsOfDirToTree(entrypath, subtree)


def upload(dirToUpload, repoName, overwrite):
    absDirToUpload = os.path.abspath(dirToUpload)

    if not os.path.isdir(absDirToUpload):
        raise IOError("\'{0}\' is not a directory".format(absDirToUpload))

    tree = nog.Tree()
    tree.name = os.path.basename(os.path.normpath(absDirToUpload))
    addContentsOfDirToTree(absDirToUpload, tree)

    try:
        repo = nog.openRepo(repoName)
    except RuntimeError:
        print('Repo \'{0}\' does not exist, let\'s create it.'.format(repoName))
        repo = nog.createRepo(repoName)

    master = repo.getMaster()
    root = master.tree
    for idx, t in root.enumerateEntries(tree.name):
        if overwrite:
            print('Repo \'{0}\' already contains entry \'{1}\'. Removing it.'.format(repoName, tree.name))
            root.pop(idx)
        else:
            raise RuntimeError('Repo \'{0}\' already contains entry \'{1}\'. Use --force-overwrite to overwrite.'.format(repoName, tree.name))
    root.append(tree)
    repo.commitTree("Dataset upload", root, master.sha1)
    print('Upload complete.')


if __name__ == '__main__':
    arguments = docopt(__doc__)
    upload(arguments['<dir-to-upload>'], arguments['<repository-name>'], arguments['--force-overwrite'])

