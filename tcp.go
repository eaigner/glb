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
	nodes       []NodeAddr
}

// TCPBalanceFunc returns a backend for the incoming connection.
type TCPBalanceFunc func(nodes []NodeAddr, conn net.Conn) NodeAddr

func NewTCPBalancer(addr string, f TCPBalanceFunc) Balancer {
	if f == nil {
		f = NewTCPRoundRobin()
	}
	return &tcpBalancer{
		addr:        addr,
		balanceFunc: f,
	}
}

func (b *tcpBalancer) SetNodes(nodes []NodeAddr) {
	b.nodeList.set(nodes)
	b.nodes = b.nodeList.get()
}

func (b *tcpBalancer) AddNode(node NodeAddr) {
	b.nodeList.add(node)
	b.nodes = b.nodeList.get()
}

func (b *tcpBalancer) Addr() net.Addr {
	return b.ln.Addr()
}

func (b *tcpBalancer) Serve(ready chan bool) error {
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
	if ready != nil {
		ready <- true
	}
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			return err
		}
		go b.handleConn(conn)
	}
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
	lastNode NodeAddr
}

func (rr *tcpRoundRobin) balance(nodes []NodeAddr, conn net.Conn) NodeAddr {
	return roundRobin(nodes, &rr.lastNode)
}

func NewTCPRoundRobin() TCPBalanceFunc {
	rr := &tcpRoundRobin{}
	return TCPBalanceFunc(rr.balance)
}