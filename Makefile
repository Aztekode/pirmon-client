build-64:
	GOARCH=amd64 go build -o pirmon-client.exe main.go client.go service.go config.go monitor.go printer.go
	GOARCH=amd64 go build -o install.exe install.go
build-32:
	GOARCH=386 go build -o pirmon-client.exe main.go client.go service.go config.go monitor.go printer.go
	GOARCH=386 go build -o install.exe install.go
install:
	install.exe pirmon-client.exe
initialize:
	net start pirmon-client
