#!/bin/bash

export JAVA_HOME="/usr/lib/jvm/java-17-openjdk-amd64"
export PATH="${JAVA_HOME}/bin:${PATH}"

# Ensure jail directory exists
mkdir -p /tmp/goboxd_jails

exec "$@"
