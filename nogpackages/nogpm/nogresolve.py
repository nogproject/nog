#!/usr/bin/env python3

# Import only packages that are usually available (preferrably only core
# packages), so that the file is self-contained and can, in principle, be used
# to resolve dependencies when dependencies such as nogpy are not yet
# available.

from glob import glob
import json
import os.path


class Resolver:
    def __init__(self, warn=None):
        self.registry = {}
        self.warn = warn or False

    def resolve(self, name):
        return self.registry[name]['localpath']

    def addPackage(self, abspath):
        if os.path.isdir(abspath):
            abspath = abspath + '/nogpackage.json'
        with open(abspath) as fp:
            nogpackage = json.load(fp)
            nogpackage['localpath'] = os.path.dirname(abspath)
            name = nogpackage['package']['name']
            if name in self.registry:
                if nogpackage['localpath'] == self.registry[name]['localpath']:
                    return
                if not self.warn:
                    return
                msg = ('Warning: Duplicate package `{0}` at `{1}`; '
                       'previous at `{2}`.')
                print(msg.format(name, nogpackage['localpath'],
                                 self.registry[name]['localpath']))
                return
            self.registry[name] = nogpackage

    def addWithParents(self, abspath):
        for p in glob(abspath + '/nogpackage.json'):
            self.addPackage(p)

        while abspath != '/':
            for p in glob(abspath + '/nogpackages/*/nogpackage.json'):
                self.addPackage(p)
            (abspath, tail) = os.path.split(abspath)

    def addWithLocal(self, abspath):
        for p in glob(abspath + '/nogpackage.json'):
            self.addPackage(p)
        self.addLocal(abspath)

    def addLocal(self, abspath):
        for p in glob(abspath + '/nogpackages/*/nogpackage.json'):
            self.addPackage(p)
