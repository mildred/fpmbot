export ANSIBLE_NOCOWS=1
ANSIBLEFLAGS=-i hosts.sample
DESTDIR=
INSTALL_METHOD=system

-include config.mk

all:
	$(MAKE) build
	$(MAKE) fpmbot.tar
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

ifeq ($(INSTALL_METHOD),system)
install:
	mkdir -p $(DESTDIR)/usr/bin
	install -m755 fpmbot $(DESTDIR)/usr/bin/fpmbot
	install -m755 fpmbot-inotify $(DESTDIR)/usr/bin/fpmbot-inotify
	mkdir -p $(DESTDIR)/usr/lib/systemd/system
	install -m644 fpmbot.timer $(DESTDIR)/usr/lib/systemd/system/fpmbot.timer
	install -m644 fpmbot.service $(DESTDIR)/usr/lib/systemd/system/fpmbot.service
	install -m644 fpmbot-inotify.service $(DESTDIR)/usr/lib/systemd/system/fpmbot-inotify.service
endif

ifeq ($(INSTALL_METHOD),docker)
install:
	mkdir -p $(DESTDIR)/usr/lib/fpmbot
	cp fpmbot.tar $(DESTDIR)/usr/lib/fpmbot/fpmbot.tar
endif

.PHONY: all help build fpmbot.tar install-testrepo install-fpmbot install
