# seeshell

seeshell lets a user output their current terminal output to a web interface using nothing other than a net client (like netcat)

## Deploy

## CLI Flags

```text
The seeshell command

Usage:
  seeshell [flags]

Flags:
  -c, --config string                    Config file (default "config.yml")
      --data-directory string            Directory that holds data (default "deploy/data/")
      --debug                            Enable debugging information
  -h, --help                             help for seeshell
      --http-address string              HTTP/WS service address (default "localhost:8080")
      --http-domain string               The domain for the service to be outputted (default "localhost")
      --http-port int                    The http port to display in command output (default 8080)
      --https-enabled                    Whether HTTPS is enabled (reverse proxy)
      --log-to-file                      Enable writing log output to file, specified by log-to-file-path
      --log-to-file-compress             Enable compressing log output files
      --log-to-file-max-age int          The maxium number of days to store log output in a file (default 28)
      --log-to-file-max-backups int      The maxium number of rotated logs files to keep (default 3)
      --log-to-file-max-size int         The maximum size of outputed log files in megabytes (default 500)
      --log-to-file-path string          The file to write log output to (default "/tmp/seeshell.log")
      --log-to-stdout                    Enable writing log output to stdout (default true)
      --secret-path string               The path used to print session ids. An empty string is used to disable this
      --tcp-address string               TCP service address (default "localhost:8081")
      --tcp-transparent-address string   TCP transparent address (default "localhost:8082")
      --time-format string               The time format to use for general log messages (default "2006/01/02 - 15:04:05")
  -v, --version                          version for seeshell
```
