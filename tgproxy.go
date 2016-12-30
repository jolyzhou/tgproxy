package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
)

const bufferSize = 8192

// TGProxyServer struct contain transport type host port
type TGProxyServer struct {
	svrType string
	svrHost string
	svrPort string
}

type proxyHost struct {
	host    string
	port    string
	isHTTPS bool
}

// init socket server
func (tg *TGProxyServer) init() net.Listener {

	flag.StringVar(&tg.svrType, "t", "tcp", "transport type tcp or udp")
	flag.StringVar(&tg.svrHost, "h", "127.0.0.1", "listen host")
	flag.StringVar(&tg.svrPort, "p", "8989", "listen port")
	flag.Parse()

	fmt.Println("=========tgproxy 0.0.1 is running=========")
	svr, err := net.Listen(tg.svrType, tg.svrHost+":"+tg.svrPort)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	return svr
}

// accept client
func (tg *TGProxyServer) acceptClient(server net.Listener) chan net.Conn {
	channel := make(chan net.Conn)

	go func() {
		for {
			client, err := server.Accept()
			if client == nil {
				fmt.Println("Can not accept : ", err.Error())
				continue
			}
			channel <- client
		}

	}()

	return channel
}

// rewrite Header
func (tg *TGProxyServer) rewriteHeaderBuffer(headerBuff []byte, hosts proxyHost) []byte {
	if hosts.isHTTPS == true {
		return headerBuff
	}
	if hosts.port == "80" {
		r := strings.NewReplacer("http://"+hosts.host, "")
		return []byte(r.Replace(string(headerBuff)))
	}

	r := strings.NewReplacer("http://"+hosts.host+":"+hosts.port, "")
	return []byte(r.Replace(string(headerBuff)))

}

// get buffer from socket
func (tg *TGProxyServer) getHeaderBuffer(client net.Conn) []byte {
	buffer := make([]byte, bufferSize)
	client.Read(buffer)
	return buffer
}

// parse HTTP Header
func (tg *TGProxyServer) parseHTTPHeader(headerBuffer string) (hosts proxyHost, err error) {
	headURL := strings.SplitN(headerBuffer, "\r\n", 2)[0]
	if strings.Index(headURL, "HTTP") == -1 {
		return hosts, errors.New("error header")
	}
	urls := strings.Split(headURL, " ")[1]
	if strings.Index(urls, "http://") < 0 {
		if strings.Index(urls, ":") != -1 {
			hosts.host = urls[:strings.Index(urls, ":")]
		}
		hosts.port = "443"
		hosts.isHTTPS = true
	} else {
		takeURL, _ := url.Parse(urls)
		if strings.Index(takeURL.Host, ":") > 0 {
			hosts.host = strings.Split(takeURL.Host, ":")[0]
			hosts.port = strings.Split(takeURL.Host, ":")[1]
		} else {
			hosts.host = takeURL.Host
			hosts.port = "80"
		}
		hosts.isHTTPS = false
	}
	return hosts, nil
}

// connect host by socket
func (tg *TGProxyServer) connectHost(client net.Conn) {
	defer client.Close()
	buffer := tg.getHeaderBuffer(client)
	hosts, err := tg.parseHTTPHeader(string(buffer))
	if err != nil {
		return
	}
	rwBuffer := tg.rewriteHeaderBuffer(buffer, hosts)
	connectionHost, _ := net.Dial(tg.svrType, hosts.host+":"+hosts.port)
	defer connectionHost.Close()
	if hosts.isHTTPS == true {
		client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		tg.pipe(connectionHost, client)
	} else {
		connectionHost.Write(rwBuffer)
		tg.pipe(connectionHost, client)
	}
	return
}

// pipe socket
func (tg *TGProxyServer) pipe(des, src net.Conn) {
	go func() {
		io.Copy(des, src)
	}()
	io.Copy(src, des)
}

func main() {
	tgproxyServer := &TGProxyServer{}

	server := tgproxyServer.init()
	defer server.Close()

	connections := tgproxyServer.acceptClient(server)

	for {
		go tgproxyServer.connectHost(<-connections)
	}
}
