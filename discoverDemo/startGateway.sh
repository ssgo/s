#!/bin/sh

echo "Test Use:\n\tcurl 'http://localhost:8080/Andy'\nCrtl+C to exit\n"

export SERVICE_LISTEN=:8080
export SERVICE_CALLS='{"s1": {"accessToken": "aabbcc"}}'
go run gateway.go
