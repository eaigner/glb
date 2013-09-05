package glb

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestHTTP(t *testing.T) {
	createSrv := func(msg string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprint(w, msg)
		}))
	}

	srv1 := createSrv("hello from srv1")
	srv2 := createSrv("hello from srv2")

	defer srv1.Close()
	defer srv2.Close()

	addr1 := srv1.Listener.Addr().String()
	addr2 := srv2.Listener.Addr().String()

	lb := NewHTTPBalancer("127.0.0.1:0", nil, NewHTTPRoundRobin())
	lb.AddNode(NodeAddr(addr1))
	lb.AddNode(NodeAddr(addr2))

	ready := make(chan bool)
	go func() {
		err := lb.Serve(ready)
		if err != nil {
			t.Fatal(err)
		}
	}()
	<-ready

	lbAddr := lb.Addr().String()
	url := "http://" + lbAddr + ""

	t.Log("srv1:", addr1)
	t.Log("srv2:", addr2)
	t.Log("lb:", lbAddr)
	t.Log("url:", url)

	a := make([]string, 0, 3)

	call := func() {
		res, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}

		greeting, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			panic(err)
		}

		a = append(a, string(greeting))
	}

	call()
	call()
	call()

	if !reflect.DeepEqual(a, []string{
		"hello from srv1",
		"hello from srv2",
		"hello from srv1",
	}) {
		t.Fatal(a)
	}
}
