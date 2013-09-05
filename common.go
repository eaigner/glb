package glb

import (
	"io"
	"net"
	"sync"
)

type NodeAddr string

type Balancer interface {
	SetNodes(nodes []NodeAddr)
	AddNode(node NodeAddr)
	Addr() net.Addr
	Serve(ready chan bool) error
	Close() error
}

func roundRobin(b []NodeAddr, last *NodeAddr) NodeAddr {
	switch len(b) {
	case 0:
		return ""
	case 1:
		return b[0]
	}
	use := b[0]
	for i, v := range b {
		if v == *last && i < len(b)-1 {
			use = b[i+1]
			break
		}
	}
	*last = use
	return use
}

func copyAndClose(wc io.WriteCloser, r io.Reader) {
	defer wc.Close()
	io.Copy(wc, r)
}

type nodeList struct {
	nodes   []NodeAddr
	nodeMap map[NodeAddr]int
	nodeMtx sync.Mutex
}

func (l *nodeList) get() []NodeAddr {
	l.nodeMtx.Lock()
	defer l.nodeMtx.Unlock()
	v := make([]NodeAddr, len(l.nodes))
	copy(v, l.nodes)

	return v
}

func (l *nodeList) set(nodes []NodeAddr) {
	l.nodeMtx.Lock()
	defer l.nodeMtx.Unlock()
	l.nodeMap = make(map[NodeAddr]int)
	l.nodes = nil
	for _, n := range nodes {
		_, ok := l.nodeMap[n]
		if !ok {
			l.nodeMap[n] = 1
			l.nodes = append(l.nodes, n)
		}
	}
}

func (l *nodeList) add(node NodeAddr) {
	l.nodeMtx.Lock()
	defer l.nodeMtx.Unlock()
	if l.nodeMap == nil {
		l.nodeMap = make(map[NodeAddr]int)
	}
	_, ok := l.nodeMap[node]
	if !ok {
		l.nodeMap[node] = 1
		l.nodes = append(l.nodes, node)
	}
}
