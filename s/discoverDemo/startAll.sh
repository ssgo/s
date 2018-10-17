#!/bin/sh

echo "Test Use:\n\tcurl 'http://localhost:8080/Andy'\nCrtl+C to exit\n"

#export SERVICE_LOGFILE=/dev/null
export SERVICE_APP=s1
export SERVICE_ACCESSTOKENS='{"aabbcc":1}'
go run service.go &
go run service.go &

unset SERVICE_APP
unset SERVICE_ACCESSTOKENS
export SERVICE_LISTEN=:8080
export SERVICE_CALLS='{"s1": {"accessToken": "aabbcc"}}'
go run gateway.go
