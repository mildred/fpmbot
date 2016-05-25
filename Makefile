export ANSIBLE_NOCOWS=1
ANSIBLEFLAGS=-i hosts.sample
DESTDIR=
INSTALL_METHOD=system

-include config.mk

all:
ifeq ($(INSTALL_METHOD),system)
	$(MAKE) fpmbuild
	$(MAKE) fpmbot2
endif

ifeq ($(INSTALL_METHOD),docker)
	$(MAKE) build
	$(MAKE) fpmbot.tar
endif

help:
	@echo "$(MAKE) build            - build fpmbot docker image"
	@echo "$(MAKE) fpmbot.tar       - save fpmbot image to fpmbot.tar"
	@echo "$(MAKE) install-fpmbot   - Ansible install fpmbot docker image"
	@echo "$(MAKE) install-testrepo - Ansible install src/test.repo"
	@echo
	@echo "INSTALL_METHOD=system    - Install directly on the system on make install"
	@echo "INSTALL_METHOD=docker    - Install a docker container on make install"
build:
	docker build -t fpmbot .
fpmbot.tar:
	docker save -o $@ fpmbot
install-testrepo install-fpmbot:
	ansible-playbook $(ANSIBLEFLAGS) $@.yml
fpmbuild fpmbot2:
	GOPATH="$$PWD/gopath:$${GOPATH:-"$$PWD/gopath"}" go build -v -o $@ ./cmd/$@/

ifeq ($(INSTALL_METHOD),system)
install:
	mkdir -p $(DESTDIR)/usr/bin
	install -m755 fpmbot2 $(DESTDIR)/usr/bin/fpmbot2
	install -m755 fpprunerepo $(DESTDIR)/usr/bin/fpprunerepo
	install -m755 fprepo-deb $(DESTDIR)/usr/bin/fprepo-deb
	install -m755 fpmbuild $(DESTDIR)/usr/bin/fpmbuild
	install -m755 fpmbot-inotify $(DESTDIR)/usr/bin/fpmbot-inotify
	mkdir -p $(DESTDIR)/usr/lib/systemd/system
	install -m644 fpmbot.timer $(DESTDIR)/usr/lib/systemd/system/fpmbot.timer
	install -m644 fpmbot.service $(DESTDIR)/usr/lib/systemd/system/fpmbot.service
	install -m644 fpmbot-inotify.service $(DESTDIR)/usr/lib/systemd/system/fpmbot-inotify.service
	#install -m644 fpm.yaml $(DESTDIR)/etc/fpmbuild.d/fpm.yaml

.fpm: Makefile
	echo "-s dir -C fpmroot" >$@
endif

ifeq ($(INSTALL_METHOD),docker)
install:
	mkdir -p $(DESTDIR)/usr/lib/fpmbot
	cp fpmbot.tar $(DESTDIR)/usr/lib/fpmbot/fpmbot.tar

.fpm: Makefile
	: >$@
	echo "--after-install after-install.sh" >>$@
	echo "--before-remove before-remove.sh" >>$@
	echo "-s dir -C fpmroot" >>$@
endif

TARGET ?= deb
FPMBUILD_SUDO ?= -sudo
fpm: fpmbot2 fpmbuild
	PATH="$$PATH:$$PWD" ./fpmbot2 -t $(TARGET) fpm.yaml

package: fpmbuild
	./fpmbuild $(FPMBUILD_SUDO) -t $(TARGET)

.PHONY: all help build fpmbot.tar install-testrepo install-fpmbot install fpmbuild fpmbot2 fpm package
