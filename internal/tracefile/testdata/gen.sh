#!/bin/bash
for pkg in net/http sync/atomic log; do
  go test "${pkg}" -trace "$(echo $pkg|tr '/' '_').trace"
done
