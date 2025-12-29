test-win:
	GOOS=windows go build -o spectra-client.exe ./cmd/spectra-client
	cmd.exe /c spectra-client.exe