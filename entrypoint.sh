#!/bin/sh

socat TCP-LISTEN:443,fork,bind=0.0.0.0 TCP:card-web.fly.dev:8080 &

exec /usr/local/bin/run-app
