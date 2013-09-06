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
	nodes       []string
}

// HTTPBalanceFunc returns a backend for the incoming request
type HTTPBalanceFunc func(b []string, req *http.Request) string

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

func (b *httpBalancer) SetNodes(nodes []string) {
	b.nodeList.set(nodes)
	b.nodes = b.nodeList.get()
}

func (b *httpBalancer) AddNode(node string) {
	b.nodeList.add(node)
	b.nodes = b.nodeList.get()
}

func (b *httpBalancer) Addr() net.Addr {
	return b.ln.Addr()
}

func (b *httpBalancer) Listen() error {
	var err error
	b.ln, err = net.Listen("tcp", b.addr)
	if err != nil {
		return err
	}
	if b.tlsConf != nil {
		b.ln = tls.NewListener(b.ln, b.tlsConf)
	}
	return nil
}

func (b *httpBalancer) Serve() error {
	srv := &http.Server{
		Addr:    b.addr,
		Handler: http.HandlerFunc(b.handleConn),
	}
	return srv.Serve(b.ln)
}

func (b *httpBalancer) ListenAndServe() error {
	err := b.Listen()
	if err != nil {
		return err
	}
	return b.Serve()
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
	lastNode string
}

func (rr *httpRoundRobin) balance(nodes []string, req *http.Request) string {
	return roundRobin(nodes, &rr.lastNode)
}

func NewHTTPRoundRobin() HTTPBalanceFunc {
	rr := &httpRoundRobin{}
	return HTTPBalanceFunc(rr.balance)
}
