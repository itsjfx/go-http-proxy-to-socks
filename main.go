package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/proxy"
)

type Proxy struct {
	SocksHostname string
	SocksPort     string
	SocksUsername string
	SocksPassword string
	ListenIp      string
	HTTPPort      string
	Dialer        proxy.Dialer
}

// RFC2616 13.5.1
// https://datatracker.ietf.org/doc/html/rfc2616#section-13.5.1
var HOP_BY_HOP_HEADERS = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authorization",
	"Proxy-Connection",
	"TE",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeaders(src, dst http.Header) {
	for k, v := range src {
		if skipHopByHopHeader(k) {
			continue
		}
		dst[k] = v
	}
}

func skipHopByHopHeader(header string) bool {
	header = strings.ToLower(header)
	for _, h := range HOP_BY_HOP_HEADERS {
		if strings.ToLower(h) == header {
			return true
		}
	}
	return false
}

// I prefer not doing this
// but it saves a bunch of code for arg parsing
func (p *Proxy) ParseArgs() {
	flag.StringVar(&p.SocksHostname, "socks-hostname", "localhost", "SOCKS5 server hostname or IP (default: localhost)")
	flag.StringVar(&p.SocksPort, "socks-port", "1080", "SOCKS5 server port (default: 1080)")
	flag.StringVar(&p.SocksUsername, "socks-username", "", "SOCKS5 username (optional)")
	flag.StringVar(&p.SocksPassword, "socks-password", "", "SOCKS5 password (optional)")
	flag.StringVar(&p.ListenIp, "listen-ip", "0.0.0.0", "IP address to listen on (default: 0.0.0.0)")
	flag.StringVar(&p.HTTPPort, "http-port", "8080", "HTTP proxy server port (default: 8080)")
	flag.Parse()
}

func (p *Proxy) CreateDialer() {
	var auth *proxy.Auth
	if p.SocksUsername != "" && p.SocksPassword != "" {
		auth = &proxy.Auth{
			User:     p.SocksUsername,
			Password: p.SocksPassword,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", p.SocksHostname, p.SocksPort), auth, proxy.Direct)
	if err != nil {
		log.Fatalf("Failed to create SOCKS5 dialer: %v", err)
	}
	p.Dialer = dialer
}

func (p *Proxy) Start() {
	p.CreateDialer()
	listenAddress := net.JoinHostPort(p.ListenIp, p.HTTPPort)
	server := &http.Server{
		Addr:    listenAddress,
		Handler: http.HandlerFunc(p.handler),
	}

	log.Printf("Starting HTTP proxy on %s, forwarding to SOCKS5 server %s:%s", server.Addr, p.SocksHostname, p.SocksPort)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("HTTP proxy server failed: %v", err)
	}
}

func (p *Proxy) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := p.Dialer.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, "Failed to connect to target via SOCKS5", http.StatusServiceUnavailable)
		log.Printf("CONNECT error: %v", err)
		return
	}
	defer conn.Close()

	// need to hijack the client connection to get access the raw TCP connection to relay TCP traffic
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	p.copyConn(clientConn, conn)
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	var host = r.URL.Host
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "80")
	}
	conn, err := p.Dialer.Dial("tcp", host)
	if err != nil {
		http.Error(w, "Failed to connect to target via SOCKS5", http.StatusServiceUnavailable)
		log.Printf("HTTP error: %v", err)
		return
	}
	defer conn.Close()

	req, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		log.Printf("Failed to create request: %v", err)
		return
	}

	// remove hop-by-hop headers
	copyHeaders(req.Header, r.Header)

	if err := req.Write(conn); err != nil {
		http.Error(w, "Failed to forward request to ", http.StatusInternalServerError)
		log.Printf("Failed to write request to : %v", err)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), r)
	if err != nil {
		http.Error(w, "Failed to read response from ", http.StatusInternalServerError)
		log.Printf("Failed to read response from : %v", err)
		return
	}
	defer resp.Body.Close()

	// remove hop by headers again (?)
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	// TODO safe?
	io.Copy(w, resp.Body)
}

// TODO safe?
func (p *Proxy) copyConn(clientConn, targetConn net.Conn) {
	go io.Copy(targetConn, clientConn)
	io.Copy(clientConn, targetConn)
}

func main() {
	proxy := &Proxy{}
	proxy.ParseArgs()
	proxy.Start()
}
