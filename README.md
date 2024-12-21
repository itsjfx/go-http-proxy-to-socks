# go-http-proxy-to-socks

Something quick I wrote to solve an issue I had, and to learn [net Dialers](https://pkg.go.dev/net#Dial) and [http Handlers](https://pkg.go.dev/net/http#Handler) in Go.

* I didn't implement the full RFC. If you want something proper, use [tinyproxy](https://github.com/tinyproxy/tinyproxy)
* I'd not use this in a production environment
* I've used this briefly a handful of times and not faced any issues

Use case:
* `ssh -D` ("dynamic" forwarding) exposes a SOCKS server, but some applications only support HTTP proxies
* Sometimes my main workaround, [proxychains](https://github.com/rofl0r/proxychains-ng), does not work (e.g. if the binary is statically linked)
* A HTTP proxy that forwards to the SOCKS server is a simple workaround
* Go has amazing static cross-compilation support, so I can build and run this for an ARM64 system very easily

## Usage

```
Usage of ./hpts:
  -http-port string
        HTTP proxy server port (default: 8080) (default "8080")
  -listen-ip string
        IP address to listen on (default: 0.0.0.0) (default "0.0.0.0")
  -socks-hostname string
        SOCKS5 server hostname or IP (default: localhost) (default "localhost")
  -socks-password string
        SOCKS5 password (optional)
  -socks-port string
        SOCKS5 server port (default: 1080) (default "1080")
  -socks-username string
        SOCKS5 username (optional)
```

## See also

* [oyyd/http-proxy-to-socks](https://github.com/oyyd/http-proxy-to-socks)
* [chux0519/hpts](https://github.com/chux0519/hpts)
