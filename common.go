package glb

import (
	"io"
	"net"
	"sync"
)

type Balancer interface {
	SetNodes(nodes []string)
	AddNode(node string)
	Addr() net.Addr
	Serve(ready chan bool) error
	Close() error
}

var rrMtx sync.Mutex

func roundRobin(b []string, last *string) string {
	rrMtx.Lock()
	defer rrMtx.Unlock()

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
	mtx   sync.Mutex
	nodes map[string]int
}

func (l *nodeList) get() []string {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	a := make([]string, 0, len(l.nodes))
	for n, _ := range l.nodes {
		a = append(a, n)
	}
	return a
}

func (l *nodeList) set(ns []string) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.nodes = make(map[string]int)
	for _, n := range ns {
		l.nodes[n] = 1
	}
}

func (l *nodeList) add(n string) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if l.nodes == nil {
		l.nodes = make(map[string]int)
	}
	l.nodes[n] = 1
}
