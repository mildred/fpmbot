#!/bin/sh

( set -x
  docker kill fpmbot
  docker rm fpmbot
)
if docker images | grep '^fpmbot '; then
  ( set -ex
    docker rmi fpmbot
  )
fi
true
