## Compilation Instructions

### Server - Linux 64-bit (from Debian machine)

To compile the server for Linux 64-bit execution:

```bash
cd server
GOOS=linux GOARCH=amd64 go build -o server server.go
```

This will create a `server` executable that runs on Linux 64-bit systems.

### Client - Windows 64-bit (from Debian machine)

To compile the client as a 64-bit Windows executable:

```bash
cd client
GOOS=windows GOARCH=amd64 go build -o client.exe client.go
```

This will create a `client.exe` executable that runs on Windows 64-bit systems.

### Cross-Compilation Summary

Both commands use Go's environment variables for cross-compilation:
- `GOOS=linux` or `GOOS=windows` specifies the target operating system
- `GOARCH=amd64` specifies the target architecture (64-bit)
- `-o` specifies the output executable name

The compiled binaries will be ready to deploy to their respective target systems.
