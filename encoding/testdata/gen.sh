#!/bin/bash
_PATH="${PATH}"

function wsgo {
  export GOROOT_BOOTSTRAP="/one/usr/go/latest"
  export GOROOT="/one/ws/godev${1}/go"
  export GOPATH="/one/ws/godev${1}"
  export PATH="${GOPATH}/bin:${GOROOT}/bin:${_PATH}"
}

for ver in 1.5 1.7 1.8; do
  wsgo "${ver}"
  testdir="$(go version |awk '{print $3}')"
  mkdir -p "${testdir}"

  for pkg in fmt log; do
    go test "${pkg}" -trace "${testdir}/tiny_$(echo $pkg|tr '/' '_').trace"
  done
  go test "sync" -run "WaitGroup$" \
    -trace "${testdir}/tiny_sync_pool.trace"
  go test "sync" -run "Pool$" \
    -trace "${testdir}/tiny_sync_wg.trace"

  for pkg in os text/template sync/atomic go/format go/scanner go/parser go/types net/http path/filepath; do
    go test "${pkg}" -trace "${testdir}/$(echo $pkg|tr '/' '_').trace"
  done
done

rm -f *.test
mv go1.5.4 v1
mv go1.7.4 v2
mv go1.8rc1 v3
