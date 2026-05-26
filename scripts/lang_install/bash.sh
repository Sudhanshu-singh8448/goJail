#!/bin/bash

set -e

echo "Verifying Bash installation..."

if /bin/bash --version > /dev/null 2>&1; then
    echo "Bash is installed and working."
    /bin/bash -c 'echo "Bash test passed"'
else
    echo "Bash verification failed."
    exit 1
fi
