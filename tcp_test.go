package glb

import (
	"net"
	"strings"
	"testing"
)

func TestTCP(t *testing.T) {
	echoFunc := func(msg string, conn net.Conn) {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}

		echo := []byte(msg + "=" + string(buf[:n]))

		_, err = conn.Write(echo)
		if err != nil {
			panic(err)
		}
	}

	createSrv := func(msg string, ready chan net.Listener) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic(err)
		}
		ready <- ln
		for {
			c, err := ln.Accept()
			if err != nil {
				t.Log(err)
				break
			}
			go echoFunc(msg, c)
		}
	}

	lnReady := make(chan net.Listener)

	go createSrv("srv1", lnReady)
	go createSrv("srv2", lnReady)

	ln1 := <-lnReady
	ln2 := <-lnReady

	defer ln1.Close()
	defer ln2.Close()

	addr1 := ln1.Addr().String()
	addr2 := ln2.Addr().String()

	lb := NewTCPBalancer("127.0.0.1:0", NewTCPRoundRobin())
	lb.AddNode(addr1)
	lb.AddNode(addr2)

	ready := make(chan bool)
	go func() {
		err := lb.Serve(ready)
		if err != nil {
			panic(err)
		}
	}()
	<-ready

	defer lb.Close()

	lbAddr := lb.Addr().String()

	t.Log("srv1:", addr1)
	t.Log("srv2:", addr2)
	t.Log("lb:", lbAddr)

	a := make([]string, 0, 3)

	send := func(msg string) {
		conn, err := net.Dial("tcp", lbAddr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		_, err = conn.Write([]byte(msg))
		if err != nil {
			panic(err)
		}

		reply := make([]byte, 1024)
		n, err := conn.Read(reply)
		if err != nil {
			panic(err)
		}

		a = append(a, string(reply[:n]))
	}

	send("1")
	send("2")
	send("3")

	res := strings.Join(a, ",")
	exp := "srv1=1,srv2=2,srv1=3"

	if res != exp {
		t.Log([]byte(exp))
		t.Fatal([]byte(res))
	}
}
