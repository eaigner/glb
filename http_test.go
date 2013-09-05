package glb

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestHTTP(t *testing.T) {
	runHttp(t, nil)
}

func TestHTTPS(t *testing.T) {
	runHttp(t, makeTlsConf())
}

// Self signed cert and key
var tlsCertPem = []byte(`-----BEGIN CERTIFICATE-----
MIICATCCAWoCCQDHkynjn8YFuTANBgkqhkiG9w0BAQUFADBFMQswCQYDVQQGEwJB
VTETMBEGA1UECBMKU29tZS1TdGF0ZTEhMB8GA1UEChMYSW50ZXJuZXQgV2lkZ2l0
cyBQdHkgTHRkMB4XDTEzMDkwNTIyNDc0MVoXDTE0MDkwNTIyNDc0MVowRTELMAkG
A1UEBhMCQVUxEzARBgNVBAgTClNvbWUtU3RhdGUxITAfBgNVBAoTGEludGVybmV0
IFdpZGdpdHMgUHR5IEx0ZDCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAuqiL
pynTwNw3lAitgKUzAx0wRiy2Di12wEp7TzXOY9kraavo1U5OaYmUwWsFcgg2vjL7
6P4iEJwH+oKyTZ9nntDelOwx389UYPwDIrMm6p595NHr3EmeFqYrwKJoMVpAfWvC
AECGngCGDmKjasvWcd82a31qxPshxVDHqVsHx08CAwEAATANBgkqhkiG9w0BAQUF
AAOBgQB/abjl2Dh/X+8nf04PEKsxUmzW0Tpsk/qS6wuP/JgGmQKNDZOgDoyAZbUp
VVRhQSK1X5+AMc2dfrhcGV/hqpecO25xwAx0E66heCVaK/49RQotiz+O0Jm6jHNM
68v5jSi1afeZp/m7mDI5Cxy2XGgprSLOc/RpLdCnUK52eq9TpQ==
-----END CERTIFICATE-----`)

var tlsKeyPem = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQC6qIunKdPA3DeUCK2ApTMDHTBGLLYOLXbASntPNc5j2Stpq+jV
Tk5piZTBawVyCDa+Mvvo/iIQnAf6grJNn2ee0N6U7DHfz1Rg/AMisybqnn3k0evc
SZ4WpivAomgxWkB9a8IAQIaeAIYOYqNqy9Zx3zZrfWrE+yHFUMepWwfHTwIDAQAB
AoGAMwBooDVSkajaWs2AMt1wsdIg5ZvD5t3PS71OMhd+nFOzg/0f8mCiFj4scij+
5OiPpKqjoEcIIcewemeJtqHumsP5x+hJeRkoJ7lWOjC/19flZcSnBHxlq7H5Olxs
G6D5GUMGSEair1DYb3SHnxRkmGkbCU70yDvPbtrNc0KQS9kCQQDdVNKdU7XHPszj
RytJc/AOqJCy7kBn8za/Jkuf2MdGCW7NuPIeTxp/ONhwnsjD6LUCZ8Euwc7wGWKh
jRJj472lAkEA1+VeuHey2bRESfErQDYfYMRkfUbCj5AVGm5fCrsaO/Kh9x7shlU4
vkcowCb6S3fP6MCe3tlZw51BYrKTa3XG4wJBAMAMM9wzoI1MXrfvLw5DPU9a0IOR
2+zWyvA9qG0Aypho4u46xkuqU9GEX7oI7Segqj92C9gobwlC3aRUJlrqZ8kCQFOk
KzQwO3wYWLSE2IrB7RoiPARE26+e1G4vAGc54YoEEDebJWtNrPQawXDgKOv/+O5l
YadYcWxVijVglbh2Ip0CQGs+1jV+SrPBo8eX+xXC/Mk5A3BJJyVLyWzeshQwRCje
0sAzJFwjEtZyJoX1Cac7KW1/cgf7nwOD5FFTa04BmdU=
-----END RSA PRIVATE KEY-----`)

func makeTlsConf() *tls.Config {
	cert, err := tls.X509KeyPair(tlsCertPem, tlsKeyPem)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}
}

func runHttp(t *testing.T, tlsConf *tls.Config) {
	createSrv := func(msg string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			proto := req.Header.Get("X-Forwarded-Proto")
			if proto == "" {
				proto = "http"
			}
			fmt.Fprint(w, msg+"="+proto)
		}))
	}

	srv1 := createSrv("srv1")
	srv2 := createSrv("srv2")

	defer srv1.Close()
	defer srv2.Close()

	addr1 := srv1.Listener.Addr().String()
	addr2 := srv2.Listener.Addr().String()

	lb := NewHTTPBalancer("127.0.0.1:0", tlsConf, NewHTTPRoundRobin())
	lb.AddNode(addr1)
	lb.AddNode(addr2)

	ready := make(chan bool)
	go func() {
		err := lb.Serve(ready)
		if err != nil {
			t.Fatal(err)
		}
	}()
	<-ready

	defer lb.Close()

	lbAddr := lb.Addr().String()
	proto := "http"
	if tlsConf != nil {
		proto = "https"
	}
	url := proto + "://" + lbAddr + ""

	t.Log("srv1:", addr1)
	t.Log("srv2:", addr2)
	t.Log("lb:", lbAddr)
	t.Log("url:", url)

	a := make([]string, 0, 3)

	call := func() {
		client := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		res, err := client.Get(url)
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
		"srv1=" + proto,
		"srv2=" + proto,
		"srv1=" + proto,
	}) {
		t.Fatal(a)
	}
}
