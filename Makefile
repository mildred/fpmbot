export ANSIBLE_NOCOWS=1
ANSIBLEFLAGS=-i hosts.sample
DESTDIR=

-include config.mk

all:
	$(MAKE) build
	$(MAKE) fpmbot.tar
help:
	@echo "$(MAKE) build            - build fpmbot docker image"
	@echo "$(MAKE) fpmbot.tar       - save fpmbot image to fpmbot.tar"
	@echo "$(MAKE) install-fpmbot   - Ansible install fpmbot docker image"
	@echo "$(MAKE) install-testrepo - Ansible install src/test.repo"
build:
	docker build -t fpmbot .
fpmbot.tar:
	docker save -o $@ fpmbot
install-testrepo install-fpmbot:
	ansible-playbook $(ANSIBLEFLAGS) $@.yml
install:
	mkdir -p $(DESTDIR)/usr/lib/fpmbot
	cp fpmbot.tar $(DESTDIR)/usr/lib/fpmbot/fpmbot.tar

.PHONY: all help build fpmbot.tar install-testrepo install-fpmbot install
