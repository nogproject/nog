#!/usr/bin/env python3
# vim: sw=4

import nog

N = 200

def main():
    try:
        repo = nog.createRepo('test-resolveimgsrc-optimization')
    except RuntimeError:
        repo = nog.openRepo('test-resolveimgsrc-optimization')

    master = repo.getMaster()

    root = nog.Tree()
    root.name = 'root'

    lines = [
        '# {} small squares'.format(N),
        '',
    ]

    images = nog.Tree()
    images.name = 'images'
    for i in range(N):
        img = nog.Object()
        name = 'img-{}.png'.format(i)
        img.name = name
        img.blob = '/tmp/test.png'
        images.append(img)
        lines.append('<img src="./images/{}" width="20">'.format(name))

    md = nog.Object()
    md.name = 'images.md'
    md.text = '\n'.join(lines)

    root.append(md)
    root.append(images)

    repo.commitTree(subject='images', tree=root, parent=master.sha1)


main()
