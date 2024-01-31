windows:
	@echo "Building for Windows"
	GOOS=windows GOARCH=amd64 go build -o enex2paperless.exe cmd/main/main.go

	