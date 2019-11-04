#!/usr/bin/env python3

"""
The package nogjob.py helps writing nog programs that can either be executed
locally for testing or as nog batch jobs.  See README.md for overview.
"""

import nog
import sys
import os.path
import json
from datetime import datetime
from io import StringIO

xout = StringIO()


class NogJob:
    def __init__(self, parameters):
        self.params = parameters
        self.slurm = None
        self.master = None
        self.wsRepo = None
        self.context = None

        try:
            self.slurm = self.params['nog']['job']
        except:
            pass

        print('Slurm job: ', self.slurm)

        self.wsRepo = nog.openRepo(self.params['nog']['workspaceRepo'])
        self.master = self.wsRepo.getMaster()

        if self.slurm:
            if self.master.sha1 != self.params['nog']['commitId']:
                raise RuntimeError("NogJob: master does not match "
                                   "`params.commitId`.")

    def prepareComputation(self):

        root = self.master.tree

        self.context = NogContext(self.params)
        datalist = next(root.trees('datalist'))

        self.wsRepo.prefetchBlobs(blobs(datalist))
        inputData = nog.Tree()

        for d in datalist.entries():
            if d.type != 'object':
                printmsg("Warning: skipping tree input: {0}".format(d.name))
                continue
            else:
                inputData.append(d)

        return inputData

    def commitResult(self, res):
        if not res.name:
            if self.params['nog']['program']['name']:
                res.name = self.params['nog']['program']['name'].split('/')[-1]
        if not res.name:
            raise RuntimeError("Cannot set result folder name: No program name.")
        if "/" in res.name:
            raise RuntimeError("Invalid result folder name: '" + res.name + "' (must not contain '/')")

        root = self.master.tree

        if self.slurm:
            job = findJob(root, self.params['nog']['job'])
            printmsg('job', job.sha1, stringify_pretty(job.content))
            program = findProgram(root, self.params['nog']['program'])
            vPatch = program.meta['package']['frozen'][0]['patch']
            vMinor = program.meta['package']['frozen'][0]['minor']
            vMajor = program.meta['package']['frozen'][0]['major']
            res.meta['programVersion'] = '@' + str(vMajor) + '.' + str(vMinor)\
                                     + '.'  + str(vPatch)
            printmsg('program', program.sha1, stringify_pretty(
                program.content))
            res.meta['jobResult'] = {}
            res.meta['jobResult']['jobId'] = self.params['nog']['job']['id']

        try:
            results = next(root.trees('results'))
        except StopIteration:
            results = nog.Tree()
            results.name = 'results'
            root.append(results)

        progs = nog.Tree()
        progs.name = 'programs'
        if self.slurm:
            progs.append(program)
        else:
            hint = self.progIndexDoc()
            progs.append(hint)
        res.append(progs)
        replaceEntry(results, res)

        if self.slurm:
            try:
                log = next(root.trees('log'))
                addLogEntry(
                        log, auto=True,
                        description='Completed job ' + job.meta['job']['id'],
                        content='Created automatic photo gallery.'
                    )
            except StopIteration:
                pass

            job = completeJob(job)
            obj = self.jobIndexDoc()
            job.insert(0, obj)

        msg = 'Add results'
        if self.slurm:
            msg = msg + ' for job ' + self.params['nog']['job']['id']

        master = self.wsRepo.commitTree(
            subject=msg, tree=root, parent=self.master.sha1)
        printmsg(master.sha1, stringify_pretty(master.content))


    def jobIndexDoc(self):
        doc = Document('index.md')
        name = os.path.basename(self.params['nog']['program']['name'])
        doc.insertLink('Go to result', '../../results/' + name)
        obj = nog.Object()
        obj.name = doc.name
        obj.meta['content'] = doc.content

        return obj

    def progIndexDoc(self):
        doc = Document('index.md')
        doc.insertText(
                '**<span style="color:red">Warning: </span>** The current'
                ' results are created by a non-published version.')
        obj = nog.Object()
        obj.name = doc.name
        obj.meta['content'] = doc.content

        return obj


def findJob(tree, job):
    jobs = next(tree.trees('jobs'))
    j = next(jobs.trees(job['id']))
    if j.sha1 != job['sha1']:
        raise RuntimeError('job sha1 mismatch.')
    return j


def findProgram(root, sel):
    programs = next(root.trees('programs'))
    pkg = None
    for p in programs.entries():
        if p.meta['package']['name'] == sel['name']:
            pkg = p
            break
    if not pkg:
        raise KeyError('Failed to find program package.')
    prg = None
    for p in pkg.entries():
        if p.sha1 == sel['sha1']:
            prg = p
            break
    if not prg:
        raise KeyError('Failed to find program version (sha1).')
    return prg


def blobs(datalist):
    for d in datalist.entries():
        if d.type is 'object':
            yield d.blob


# An alternative would be to store the log in a blob to keep the mongodb
# smaller.
def completeJob(job):
    job.meta['job']['status'] = 'completed'
    log = nog.Object()
    log.name = 'log'
    log.meta['description'] = 'job execution log'
    log.meta['content'] = xout.getvalue()
    job.insert(0, log)

    return job


def addLogEntry(log, description, content, auto=None):
    auto = auto or True
    if auto:
        creation = 'automatic'
    else:
        creation = 'human'
    date = datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ')
    entry = nog.Object()
    entry.name = date + '.md'
    entry.meta['logentry'] = {
        'date': date,
        'creation': creation
    }
    entry.meta['description'] = description
    entry.meta['content'] = content
    log.insert(0, entry)


def replaceEntry(tree, child):
    try:
        (idx, e) = next(tree.enumerateEntries(child.name))
        tree.pop(idx)
    except StopIteration:
        idx = 0
    tree.insert(idx, child)


class NogContext:
    def __init__(self, params):
        try:
            self.jobId = params['nog']['job']['id']
            self.retryId = params['nog']['job']['retryId']
        except:
            self.jobId = None
            self.retryId = None

        self.status = 0

    def progress(self, completed, total):
        try:
            self.status = int((completed/total)*100)
            nog.postJobProgress(self.jobId, self.retryId, completed, total)
        except:
            printmsg('Warning: Ignored error to post progress.')


def stringify_pretty(d):
    return json.dumps(d, sort_keys=True, ensure_ascii=False, indent=2) + '\n'


def printmsg(*args):
    print(*args, file=xout)
    print(*args)
    sys.stdout.flush()


class Document:
    def __init__(self, docname=None):
        self.name = docname
        self.content = ''

    def insertHeader(self, header, level):
        new = ''
        for i in range(0, level):
            new = new + '#'
        new = new + ' ' + header + '\n\n'
        self.content = self.content + new

    def insertParagraph(self, paragraph):
        new = '<p>' + paragraph + '<p> \n'
        self.content = self.content + new

    def insertImage(self, imgPath, altText=''):
        new = '![' + altText + '](' + imgPath + ')\n'
        self.content = self.content + new

    def insertLineBreak(self):
        self.content = self.content + '\n\n'

    def insertText(self, text):
        self.content = self.content + text

    def insertLink(self, lnkTxt, path):
        self.content = self.content + '[**' + lnkTxt + '**](' + path + ')'
