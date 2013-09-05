package glb

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
)

type httpBalancer struct {
	nodeList
	addr        string
	ln          net.Listener
	tlsConf     *tls.Config
	balanceFunc HTTPBalanceFunc
	nodes       []NodeAddr
}

// HTTPBalanceFunc returns a backend for the incoming request
type HTTPBalanceFunc func(b []NodeAddr, req *http.Request) NodeAddr

func NewHTTPBalancer(addr string, tlsConf *tls.Config, f HTTPBalanceFunc) Balancer {
	if f == nil {
		f = NewHTTPRoundRobin()
	}
	return &httpBalancer{
		addr:        addr,
		tlsConf:     tlsConf,
		balanceFunc: f,
	}
}

func (b *httpBalancer) SetNodes(nodes []NodeAddr) {
	b.nodeList.set(nodes)
	b.nodes = b.nodeList.get()
}

func (b *httpBalancer) AddNode(node NodeAddr) {
	b.nodeList.add(node)
	b.nodes = b.nodeList.get()
}

func (b *httpBalancer) Addr() net.Addr {
	return b.ln.Addr()
}

func (b *httpBalancer) Serve(ready chan bool) error {
	defer func() {
		if ready != nil {
			ready <- true
		}
	}()
	var err error
	b.ln, err = net.Listen("tcp", b.addr)
	if err != nil {
		return err
	}
	if b.tlsConf != nil {
		b.ln = tls.NewListener(b.ln, b.tlsConf)
	}
	srv := &http.Server{
		Addr:    b.addr,
		Handler: http.HandlerFunc(b.handleConn),
	}
	if ready != nil {
		ready <- true
	}
	return srv.Serve(b.ln)
}

func (b *httpBalancer) Close() error {
	return b.ln.Close()
}

func (b *httpBalancer) handleConn(w http.ResponseWriter, req *http.Request) {
	// Handle websocket connection upgrades
	if strings.EqualFold(req.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		b.handleWebsocket(w, req)
		return
	}

	nodeAddr := b.balanceFunc(b.nodes, req)
	proxy := &httputil.ReverseProxy{
		Director: func(rp *http.Request) {
			rp.URL.Scheme = "http"
			rp.URL.Host = string(nodeAddr)
			if b.tlsConf != nil {
				rp.Header.Add("X-Forwarded-Proto", "https")
			}
		},
	}
	proxy.ServeHTTP(w, req)
}

func (b *httpBalancer) handleWebsocket(w http.ResponseWriter, req *http.Request) {
	hijack, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket: server doesn't support connection hijacking", http.StatusInternalServerError)
		return
	}

	// Hijack HTTP conn
	src, _, err := hijack.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer src.Close()

	// Dial tcp to backend
	dst, err := net.Dial("tcp", req.URL.Host)
	if err != nil {
		http.Error(w, "websocket: could not connect to backend", http.StatusServiceUnavailable)
	}
	defer dst.Close()

	// Forward original HTTP request to backend
	err = req.Write(dst)
	if err != nil {
		log.Printf("websocket: could not forward ws request to backend: %v", err)
		return
	}

	go copyAndClose(src, dst)
	go copyAndClose(dst, src)
}

type httpRoundRobin struct {
	lastNode NodeAddr
}

func (rr *httpRoundRobin) balance(nodes []NodeAddr, req *http.Request) NodeAddr {
	return roundRobin(nodes, &rr.lastNode)
}

func NewHTTPRoundRobin() HTTPBalanceFunc {
	rr := &httpRoundRobin{}
	return HTTPBalanceFunc(rr.balance)
}
