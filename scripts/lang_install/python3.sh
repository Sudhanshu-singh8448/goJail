#!/bin/bash

set -e

echo "Installing Python 3..."
apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-dev

if python3 --version >/dev/null 2>&1; then
    echo "Python 3 installation verified successfully."
    python3 -c "print('Python 3 is working correctly!')"
else
    echo "Python 3 installation verification failed."
    exit 1
fi

mkdir -p /virtualenvs
chown root /virtualenvs