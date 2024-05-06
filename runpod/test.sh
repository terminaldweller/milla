#!/usr/bin/env sh

curl -X POST https://api.runpod.ai/v2/y7s1hikgduedkr/run \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer XXX' \
  -d '{"model": "mistral", "prompt_user": "ping", "temp": 0.2}'
