echo "Test Use:\n\tcurl 'http://localhost:8080'\nCrtl+C to exit\n"

export SERVICE_LISTEN=:8080
go run hello.go
