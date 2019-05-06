#!/bin/sh

echo "Test Use:\n\tcurl 'http://localhost:8080/Andy'\nCrtl+C to exit\n"

export DISCOVER_APP=s1
export SERVICE_ACCESSTOKENS='{"aabbcc":1}'
go run service.go &
go run service.go &

export DISCOVER_APP=c1
export DISCOVER_WEIGHT=1
export DISCOVER_CALLS='{"s1": {"headers": {"Access-Token":"aabbcc"}}}'
export SERVICE_ACCESSTOKENS='{"aabbcc":1}'
go run controller.go &
go run controller.go &

unset DISCOVER_APP
unset SERVICE_ACCESSTOKENS
export DISCOVER_CALLS='{"c1": {"headers": {"Access-Token":"aabbcc"}}}'
export SERVICE_LISTEN=:8080
go run gateway.go
