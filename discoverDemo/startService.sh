#!/bin/sh

export SERVICE_APP=s1
export SERVICE_ACCESSTOKENS='{"aabbcc":1}'
go run service.go
