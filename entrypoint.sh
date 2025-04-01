#!/bin/sh

socat TCP-LISTEN:443,fork TCP:card-web.fly.dev:443 &

exec /usr/local/bin/run-app
