#!/bin/bash
export FLIP2_ENV=test
cd "$(dirname "$0")/.."
./flip2d --config config/config-test.yaml --foreground
