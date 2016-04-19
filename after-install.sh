#!/bin/sh

set -ex
docker load -i /usr/lib/fpmbot/fpmbot.tar
docker run -d \
	--name="fpmbot" \
	--restart=always \
	-v /var/log/fpmbot:/var/log/fpmbot
	-v /var/lib/fpmbot:/var/lib/fpmbot
	fpmbot

