SeeShell
========

### A quick and easy way to send your terminal out into a browser. Example is available at ss.antoniomika.me

## How to use this software

1. Use the hosted version by running `<cmd> | nc ss.antoniomika.me 30000`
2. Build and run this code. It's as easy as:
    1. `go get github.com/fatih/color`
    2. `go get github.com/gorilla/mux`
    3. `go get github.com/gorilla/websocket`
    4. `go run main.go`
3. The HTTP/WS server will be running at localhost:8080 and the TCP Server will be running at localhost:8081
    1. This is changable by setting the `--http-addr` or `--tcp-addr` flags.