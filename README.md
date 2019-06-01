seeshell
========

## What does this do?
seeshell let's a user output their current terminal output to a web interface using nothing other than a net client (like netcat)

## How does it work?
You use a pipe to redirect your terminal output through netcat (or Socat if you'd like bidirectional forwarding) and follow the URL that is outputted by the app.

## Can I run this locally?
Sure, there are prebuilt docker containers stored in DockerHub.

## ClI Flags
```
sh-3.2# ./seeshell -h
Usage of ./seeshell:
  -debug
        Whether or not to print debug info
  -httpaddr string
        HTTP/WS service address (default "localhost:8080")
  -httpdomain string
        The domain for the service to be outputted (default "localhost")
  -httpport int
        What port to display (default 8080)
  -httpsenabled
        Whether HTTPS is enabled (reverse proxy)
  -tcpaddr string
        TCP service address (default "localhost:8081")
```