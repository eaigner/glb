package main

import (
	"fmt"
	"github.com/eaigner/glb"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
)

func createEchoServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := ioutil.ReadAll(req.Body)
		fmt.Fprint(w, string(b))
	}))
}

func main() {
	srv1 := createEchoServer()
	srv2 := createEchoServer()
	srv3 := createEchoServer()

	defer srv1.Close()
	defer srv2.Close()
	defer srv3.Close()

	addr1 := srv1.Listener.Addr().String()
	addr2 := srv2.Listener.Addr().String()
	addr3 := srv3.Listener.Addr().String()

	lb := glb.NewHTTPBalancer("127.0.0.1:6666", nil, glb.NewHTTPRoundRobin())
	lb.AddNode(addr1)
	lb.AddNode(addr2)
	lb.AddNode(addr3)

	defer lb.Close()

	ready := make(chan bool)
	done := make(chan bool)

	go func() {
		err := lb.Serve(ready)
		if err != nil {
			log.Println("error:", err)
		}
		done <- true
	}()

	<-ready

	log.Printf("lb started on %s, hit me!", lb.Addr())

	<-done

	log.Print("exit")
}
