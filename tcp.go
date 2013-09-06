package glb

import (
	"log"
	"net"
)

type tcpBalancer struct {
	nodeList
	addr        string
	ln          net.Listener
	balanceFunc TCPBalanceFunc
	nodes       []string
}

// TCPBalanceFunc returns a backend for the incoming connection.
type TCPBalanceFunc func(nodes []string, conn net.Conn) string

func NewTCPBalancer(addr string, f TCPBalanceFunc) Balancer {
	if f == nil {
		f = NewTCPRoundRobin()
	}
	return &tcpBalancer{
		addr:        addr,
		balanceFunc: f,
	}
}

func (b *tcpBalancer) SetNodes(nodes []string) {
	b.nodeList.set(nodes)
	b.nodes = b.nodeList.get()
}

func (b *tcpBalancer) AddNode(node string) {
	b.nodeList.add(node)
	b.nodes = b.nodeList.get()
}

func (b *tcpBalancer) Addr() net.Addr {
	return b.ln.Addr()
}

func (b *tcpBalancer) Listen() error {
	var err error
	b.ln, err = net.Listen("tcp", b.addr)
	return err
}

func (b *tcpBalancer) Serve() error {
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			return err
		}
		go b.handleConn(conn)
	}
	return nil
}

func (b *tcpBalancer) ListenAndServe() error {
	err := b.Listen()
	if err != nil {
		return err
	}
	return b.Serve()
}

func (b *tcpBalancer) Close() error {
	return b.ln.Close()
}

func (b *tcpBalancer) handleConn(src net.Conn) {
	nodeAddr := b.balanceFunc(b.nodes, src)
	dst, err := net.Dial("tcp", string(nodeAddr))
	if err != nil {
		src.Close()
		log.Printf("lb:tcp: could not forward to %s: %v", nodeAddr, err)
		return
	}
	go copyAndClose(src, dst)
	go copyAndClose(dst, src)
}

type tcpRoundRobin struct {
	lastNode string
}

func (rr *tcpRoundRobin) balance(nodes []string, conn net.Conn) string {
	return roundRobin(nodes, &rr.lastNode)
}

func NewTCPRoundRobin() TCPBalanceFunc {
	rr := &tcpRoundRobin{}
	return TCPBalanceFunc(rr.balance)
}
