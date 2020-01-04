ostree-upload
=============

[![License](https://img.shields.io/badge/license-GPLv3.0-blue.svg)](https://www.gnu.org/licenses/gpl-3.0.html)
[![GitHub release](https://img.shields.io/github/release/lirios/ostree-upload.svg)](https://github.com/lirios/ostree-upload)
[![Build Status](https://travis-ci.org/lirios/ostree-upload.svg?branch=develop)](https://travis-ci.org/lirios/ostree-upload)
[![GitHub issues](https://img.shields.io/github/issues/lirios/ostree-upload.svg)](https://github.com/lirios/ostree-upload/issues)

ostree-upload provides two programs:

 * ostre-receive: A server that lets you upload missing objects
   of an OSTree repository.
 * ostree-upload: A client that uploads missing objects of one
   or more OSTree branches.

This program uses Rust logging that you can configure with the `RUST_LOG`
environment variable, please check [this](https://doc.rust-lang.org/1.1.0/log/index.html) out.

## Dependencies

You need Rust and Cargo installed.

Everything works with the stable version of Rust, so you can get it from
[rustup](https://github.com/rust-lang/rustup.rs) or your distribution.

On Fedora:

```sh
sudo dnf install -y cargo
```

This programs also use the OSTree library:

```sh
sudo dnf install -y ostree-devel
```

## Installation

Build with:

```sh
cargo build
```

Run with (it needs root privileges):

```sh
sudo cargo run
```

## Configuration

`ostree-receive` reads the `config.json` file on startup in the current
directory, but the `OSTREE_RECEIVE_CONFIG` environment variable can be
set to a different file.  If you have a `.env` file in the current
directory or one of its parents, it will be read and used to
initialize environment variables.

You can also pass the configuration file to the `--config` argument,
which has precedence over the environment variable.

Here's an example of `config.json`:

```json
{
  "host": "127.0.0.1",
  "port": 8080,
  "repo-path": "/path/to/repo"
}
```

## Licensing

Licensed under the terms of the GNU General Public License version 3 or,
at your option, any later version.
