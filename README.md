FPM Bot
=======

This is a build bot for the [Effing Package Manager](http://github.com/jordansissel/fpm). For the moment it only build debian packages, but it can be extended to build anything fpm can.

The goal is to generate a package repository that can be included right next to your distribution's repositories. This way, you can create package, and manage your servers like this.

Fpmbit is designed to be run as a Docker container. See `make help` for information about Docker build process and Ansible deployment. The container provides two volumes:

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

