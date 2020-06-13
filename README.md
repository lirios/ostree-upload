<!--
SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>

SPDX-License-Identifier: CC0-1.0
-->

ostree-upload
=============

[![License](https://img.shields.io/badge/license-AGPLv3.0-blue.svg)](https://www.gnu.org/licenses/agpl-3.0.html)
[![GitHub release](https://img.shields.io/github/release/lirios/ostree-upload.svg)](https://github.com/lirios/ostree-upload)
[![CI](https://github.com/lirios/ostree-upload/workflows/CI/badge.svg?branch=develop)](https://github.com/lirios/ostree-upload/actions?query=workflow%3ACI)
[![GitHub issues](https://img.shields.io/github/issues/lirios/ostree-upload.svg)](https://github.com/lirios/ostree-upload/issues)

`ostree-upload` push commits from a local OSTree repository to a remote one,
using an HTTP API.

`ostree-upload` provides three subcommands:

 * **gentoken**: Generate an API token (more on that later)
 * **receive**: An HTTP server that lets you upload missing objects
   of an OSTree repository.
 * **push**: An HTTP client that uploads missing objects of one
   or more OSTree branches.

## Dependencies

You need Go installed.

On Fedora:

```sh
sudo dnf install -y golang
```

This programs also use the OSTree library:

```sh
sudo dnf install -y ostree-devel
```

Download all the Go dependencies:

```sh
go mod download
```

## Build

Build with:

```sh
make
```

## Install

Install with:

```sh
make install
```

The default prefix is `/usr/local` but you can specify another one:

```sh
make install PREFIX=/usr
```

And you can also relocate the binaries, this is particularly
useful when building packages:

```
...

%install
make install DESTDIR=%{buildroot} PREFIX=%{_prefix}

...
```

## Why

Usually you have separate build and production repositories, only the production
repository is accessed by the clients.

From time to time you want to copy only a few specific commits from the build
repository to the production repository, effectively publishing production
ready branches.

OSTree only allows you to pull commits and have no official push feature.

Possible solutions are:

 * pull from another local repository: this means that we have the build and
   production repositories on the same server;
 * pull from a remote repository via http: the pull has to be initiated from
   the production server and the build repository must be exposed to the
   rest of the world;
 * use another mechanism like [rsync](https://github.com/ostreedev/ostree-releng-scripts/blob/master/rsync-repos):
   the transfer is initiated by the build server via SSH, the production server must
   be accessible using SSH and there are a few limitations discussed below.

`rsync` limitations include:

* It will upload the objects directly into the store, rather than using a
  staging directory.  An interrupted transfer could leave partial objects
  in the store;

* Objects are published in sort order, meaning that objects can be published
  before their children and the refs might be updated before the
  corresponding objects are uploaded.  A client pulling during a `rsync`
  transfer may pull an incomplete or entirely missing commit.

None of these methods satisfy what we want:

 * Have build and production repositories in two different servers,
   no builds in a public server
 * Private build server that is not exposed to the Internet
 * No SSH access to the production repository needed
 * Reliable publishing mechanism

## How

The client asks the server the list of its branches from the build repository,
with the HEAD commit for each of them.

The client checks which branch are behind compared to the version
available in the production repository.

The client lists all the objects in the production repository for the commits
that are going to be pushed.  This list is compared to the list of objects
already uploaded to the build server.

This way only the missing objects are upload.

Once all objects are uploaded, the refs in the production repository are
changed to point to the new commit.

## Prior art

The concept behind this program was slightly inspiered by [ostree-push](https://github.com/dbnicholson/ostree-push),
which appears to be abandoned nowadays.

## Token

All requests to the API require a token. You can generate one with:

```sh
ostree-upload gentoken [--config=<FILENAME>]
```

This command will generate a new token and store it in the YAML file `<FILENAME>`.
The file name is `ostree-upload.yaml` by default (that is when `--config` is not passed).

If you instead wants to use Docker type something like:

```sh
docker run --rm -it \
  -v $(pwd)/ostree-upload.yaml \
  liriorg/ostree-upload \
  gentoken
```

## Server

Start the server with:

```sh
ostree-upload receive [--config=<FILENAME>] [--repo=<REPO>] [--verbose] --address=[<ADDR>]
```

This command will start the HTTP server.

The tokens are validated against the configuration file, see the previous
chapter for more information.

Replace `<REPO>` with the path to your OSTree repository, by default
it's `repo` from the current working directory.

Replace `<ADDR>` with the host name and port to bind, by default it's ":8080"
which means port `8080` on `localhost`.

Pass `--verbose` to print more messages.

If you instead wants to use Docker type something like:

```sh
docker run --rm -it \
  -v $(pwd)/ostree-upload.yaml:/etc/ostree-upload.yaml \
  -v $(pwd)/repo:/var/repo \
  -p 8080:8080 \
  liriorg/ostree-upload \
  receive -c /etc/ostree-upload.yaml -r /var/repo
```

## Client

Start the client with:

```sh
ostree-upload upload [--repo=<REPO>] [--token=<TOKEN>] [--address=<ADDR>] [[--branch=<BRANCH>], ...] [--verbose]
```

This command will upload the objects from the OSTree repository `<REPO>` to the one served
at `<ADDR>`, using the `<TOKEN>` API token.

Replace `<BRANCH>` with the branch whose objects will be uploaded.

Pass `--verbose` to print more messages.

If you instead wants to use Docker type something like:

```sh
docker run --rm -it \
  -v $(pwd)/ostree-upload.yaml:/etc/ostree-upload.yaml \
  -v $(pwd)/repo:/var/repo \
  liriorg/ostree-upload \
  push --token=<TOKEN> -c /etc/ostree-upload.yaml -r /var/repo
```

## Licensing

Licensed under the terms of the GNU Affero General Public License version 3 or,
at your option, any later version.
