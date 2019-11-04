# Nog installer

## Introduction

This is the nog installer.

To install a basic nog dev environment, download the latest installer archive
`content.tar.xz` (usually from
<https://nog.zib.de/nog/packages/files/programs/noginstaller/index!0/content.tar.xz>).

To run the installer, you need to setup the environment variables
`NOG_API_URL`, `NOG_KEYID`, `NOG_SECRETKEY`, and `NOG_CACHE_PATH` (see
<https://nog.zib.de/nog/packages/files/programs/nogpy/index!0>).

You may also need to create a Python 3 virtualenv and install packages
(typically `pip docopt`).  The installer will tell you if Python packages are
missing.

Assuming the downloaded archive is in the current working directory, unpack the
archive, execute the installer script, and clean up with:

```bash
mkdir /tmp/noginstaller
tar -C /tmp/noginstaller -xvf content.tar.xz
/tmp/noginstaller/install-nog
rm -rf /tmp/noginstaller
```

`install-nog` installs the nog package manager as `~/nogpackages/nogpm/nogpm`
in your home directory.  Consider creating a symlink in `~/bin`.

`nogpm` can then be used to install further packages.

## Technical

`noginstaller` uses symlinks to create a self-contained version of nogpm (as
`_nogpm`), which is called to install nogpm as a package with dependencies.

## Changelog

noginstaller-0.0.15, 2016-06-15:

* Updated nogpy-0.0.13 and nogpm-0.0.17 to fix potential Unicode issues.

noginstaller-0.0.14:

* Adjust links to the new convention of flagging indices in paths

noginstaller-0.0.13:

* Print instructions to update or re-install nogpackages, if there already is
  an installation available.

noginstaller-0.0.12:

* Use current nog.py

noginstaller-0.0.9:

* The package default location changed to `nog/packages`.  Previous versions
  are in `sprohaska/nogpackages`.

noginstaller-0.0.7:

* Check Python 3 executable and dependency `requests`.

noginstaller-0.0.6:

* Install nogpm with frozen deps to avoid potential problems when nogpy
  changes.

noginstaller-0.0.5:

* Instructions mention environment variables.

noginstaller-0.0.4:

* Installer checks required python imports.
* More detailed install instructions.

noginstaller-0.0.3:

* Use `nogpm link` to resolve `nog.py`.

noginstaller-0.0.2:

* Replaced the special installer script by a self-contained bootstrap version
  of nogpm.
