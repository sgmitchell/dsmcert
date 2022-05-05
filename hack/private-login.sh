#!/bin/bash

export GOPRIVATE="github.com/sgmitchell/"
echo "$GITHUB_TOKEN"
if [ -z "$GITHUB_TOKEN" ]; then
  echo "no GITHUB_TOKEN provided"
else
    line="machine github.com login ${GITHUB_TOKEN}"
    if ! grep "$line" ~/.netrc; then
      echo "$line" >> ~/.netrc
    fi
fi