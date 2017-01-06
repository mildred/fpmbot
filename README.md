FPM Bot
=======

This is a build bot for the [Effing Package Manager](http://github.com/jordansissel/fpm). For the moment it only build debian packages, but it can be extended to build anything fpm can.

The goal is to generate a package repository that can be included right next to your distribution's repositories. This way, you can create package, and manage your servers like this.

Fpmbot is designed to be run as a Docker container. See `make help` for information about Docker build process and Ansible deployment. The container provides two volumes:

- `/var/log/fpmbot`: the logs
- `/var/lib/fpmbot`: the main directory

The main directory is composed of different folders:

- `web`: contains the publicly accessible files (repositories generated) to be served by a HTTP server
- `web/debian`: debian repositories
- `web/debian/NAME.TIMESTAMP`: repository build at `TIMESTAMP`
- `web/debian/NAME`: symbolic link to the latest timestamp
- `src/*.repo`: repositories description files
- `src/PACKAGE`: source for the `PACKAGE`

Once it is run, it will build each repository every 6 hours or when an inotify event happens in `src/`. Only packages that have changed will be built. The list of packages that a repository can contain is described in `*.repo` files, the basename is the name of the repository.

A description file is line based. Comments are full lines starting with `#`, and line continuation is recognized by having the continuing line beginning with a TAB character. Each line is split on TAB character, and each element is then evaluated. The first element is the package name, ther others are in the form `key=value`. You can use:

- `git=` (optional): Git url
- `clean=` (optional): Git clean options
- `ref=` (optional): Git reference to build against
- `preparecmd=` (optional): Command to execute before build (apt-get for example)
- `fpmbuild=` (optional): build command, defaults to `make && make DESTDIR=$PWD/fpmroot install`
- `fpmgen=` (optional): command to generate `.fpm`, defaults to the contents of the `.fpmgen` file

A source package is always a Git repository. The package version is taken from `git describe`, and special files in the repository tells how to build the package:

- `.fpmgen`: how to generate the `.fpm` file
- `.fpm`: interpreted directly by fpm, the flags to pass to fpm to build the package

fpmbot2
=======

The version 2 of this bot is written in go rathen than bash, and is available
here as well. The design is different. Is it designed to be executed outside of
any container on the host system. However, it can spawn a different container
for each package it builds. Package building still occurs in a container, but
each package can have a different environment.

Also, the configuration is in YAML.

A repository configuration looks like:

    --- 
    target: <fpm target, optional, "deb", "rpm", ...>
    packages:
      package_name: <fpmbuild package description>

The target specified on the command line overrides the target specified on the
reposuitory file.

The fpmbuild package description is extended with the following keys:

- `git`: the Git repository URL
- `ref`: the Git reference to fetch. Default is `HEAD`.
- `dir`: allow to specify a subdirectory of the repository from which to create
  the package

Ideas for the future
--------------------

Use `flatpak-builder --run` to build package as simple user and not as root.

FPM Build
=========

This is the component of fpmbot2 that is responsible for building and packaging
individual packages. A package consists of a source directory.

The file `.fpmbuild.yaml` can optionally be present in that directory
containing for example:

    --- 
    # Build commands. Optional, the defaults are presented here:
    build:
      # These commands are executed in this order. Each one can be individually
      # overriden:
      prepare:
      build:   if [ -e Makefile ]; then make DESTDIR="$PWD/fpmroot"; fi
      fpmgen:  if [ -e Makefile ]; then make DESTDIR="$PWD/fpmroot" .fpm || true; fi
      install: if [ -e Makefile ]; then rm -rf fpmroot; make DESTDIR="$PWD/fpmroot" install; fi
      # Shell:
      shell:     sh
      options:   ["-c", "-xe"]
      arguments: []

    # Environment specification: if not specified, runs on the host
    env:
      docker:
        # Image specification
        Dockerfile: |
          FROM debian:testing
          ENV DEBIAN_FRONTEND noninteractive
          RUN apt-get update && apt-get install -y build-essential golang
        # image name. Incompatible with Dockerfile
        image: debian:stable
        # source directory where the package is build. Optional. Default is /src
        srcdir: /src

    # Request a git clean if not empty (default is empty)
    clean: -fdx

    # Additional fpm options (default is empty). Replaces the need for a .fpm
    # file
    fpm: ["-s", "dir", "myapp=/usr/bin/myapp"]

    # Additional FPM Hooks (default is empty)
    fpm-hooks:
        before-install: |
            #!/bin/bash
            echo Before install

The `.fpm` file must be present (or generated) and contains the FPM command line
arguments to build the paclage. FPM is executed outside of the build
environment, so paths it contains must be relative.

Using the environment variable `FPMOPTS`, the following flags will be set to
sane defaults:

- `--name`: set to the package name, the current directory name
- `--version`: set to the package version, taken from Git

A common pattern is to have the build commands install everything in `./fpmroot`
and then use the following fpm arguments: `-s dir -C fpmroot`

FPRepo
------

The scripts `fprepo-<format>` are scripts that creates a repository in the
current directory from scanned packages.

fpprunerepo
-----------

Prune old repositories in current directory. Repository name is the sole
argument and it only heep the most recent 10 repositories.

A repository is a directory matching `$1.[0-9]*`

Roadmap
-------

Externalise the distribution repository build to external services. A
debian-repository service must be running elsewhere which provides a public
interface to download the packages, and an API-Key protected interface to create
new releases, add package to them, and make the release.

Then, the yaml file describing the repository will just need to give the address
and API key to that service in order to publich the packages. In the future it
should not rely so much on systemd timer and inotify events, but instead be a
service of its own with a webhook that can be triggered to build a repository.

Bootstrapping
=============

You must build the packages on a machine with:

- go language compiler
- ruby
- ruby gems
- fpm (which is a ruby gem)

Then:

    make fpm

will generate you packages to have fpm on your build server. This might not work, if so, just run on the server:

    gem install fpm

Then, on the build machine:

    make package

will generate you the fpmbot package for your build server. Install all those packages on your build server and you are set. You can use fpmbot to generate updates for these packages if you wish.
