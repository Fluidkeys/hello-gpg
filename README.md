# Fluidkeys command line

[![Build Status](https://travis-ci.org/fluidkeys/fluidkeys.svg?branch=master)](https://travis-ci.org/fluidkeys/fluidkeys)

Fluidkeys helps teams protect themselves with strong encryption. It builds on the OpenPGP standard and is compatible with other OpenPGP software.

Fluidkeys is licensed under the [AGPLv3](https://www.gnu.org/licenses/agpl-3.0.txt).

0.2 helps you create a key or import one from `gpg`, then automatically maintain it.

Once installed, run it with `fk`.

## Install

Head over to [download.fluidkeys.com](https://download.fluidkeys.com)

## Install from source

You'll need the [Go compiler](https://golang.org/dl/)

Clone the repo:

```
REPODIR=$(go env GOPATH)/src/github.com/fluidkeys/fluidkeys

git clone https://github.com/fluidkeys/fluidkeys.git $REPODIR
cd $REPODIR
```

Build and install to `/usr/local/bin/fk`:

```
make && sudo make install
```

If you prefer to run without `sudo` (root), install into `$HOME/bin/fk`:

```
PREFIX=$HOME make install
```

## Develop

If you want to hack on Fluidkeys locally you'll need [Go 1.10+](https://golang.org/dl/) and [dep](https://github.com/golang/dep#installation).

Run:

```
make run
```
