build:
	go build -o pirmon-client.exe main.go client.go service.go config.go monitor.go
	go build -o install.exe install.go
install:
	install.exe pirmon-client.exe
initialize:
	net start pirmon-client
