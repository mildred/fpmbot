export ANSIBLE_NOCOWS=1
ANSIBLEFLAGS=-i hosts.sample

-include config.mk

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

.PHONY: help build fpmbot.tar install-testrepo install-fpmbot
