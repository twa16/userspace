/*
 * Copyright 2017 Manuel Gauto (github.com/twa16)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
*/

package userspaced

import (
	"time"
	"net"
	"bytes"
	"encoding/hex"
	"flag"
	"strconv"
	"io"
	"github.com/op/go-logging"
	"fmt"
)

var logproxy = logging.MustGetLogger("userspace-daemon")

type ProxyInstance struct {
	ID             uint `gorm:"primary_key" json:"-"`     // Primary Key and ID of container
	PublicID       string `gorm:"index" json:"space_id"`  // Public UUID of this Space
	CreatedAt      time.Time `json:"-"`                   // Creation time
	ListenAddress  string `unique_index:idx_lstnaddress"` // Address that the proxy will listen on
	ListenPort     int `unique_index:idx_lstnaddress"`    // Port that the proxy will listen on
	ConnectAddress string                                 // Address that the proxy will connect to
	ConnectPort    int                                    // Port that the proxy will connect to
	SpaceID	       string
}

func (proxy ProxyInstance) proxyConn(conn *net.TCPConn) {
	rAddr, err := net.ResolveTCPAddr("tcp", buildConnectAddress(proxy))
	fmt.Println(rAddr)
	if err != nil {
		panic(err)
	}

	rConn, err := net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		panic(err)
	}
	defer rConn.Close()

	buf := &bytes.Buffer{}
	for {
		data := make([]byte, 256)
		n, err := conn.Read(data)
		if err != nil {
			panic(err)
		}
		buf.Write(data[:n])
		if data[0] == '\r' && data[1] == '\n' {
			break
		}
	}

	if _, err := rConn.Write(buf.Bytes()); err != nil {
		panic(err)
	}
	logproxy.Debugf("sent:\n%v", hex.Dump(buf.Bytes()))

	data := make([]byte, 1024)
	n, err := rConn.Read(data)
	if err != nil {
		if err != io.EOF {
			panic(err)
		} else {
			logproxy.Debugf("received err: %v", err)
		}
	}
	logproxy.Debugf("received:\n%v", hex.Dump(data[:n]))
}

func (proxy ProxyInstance) handleConn(in <-chan *net.TCPConn, out chan<- *net.TCPConn) {
	for conn := range in {
		proxy.proxyConn(conn)
		out <- conn
	}
}

func (proxy ProxyInstance) closeConn(in <-chan *net.TCPConn) {
	for conn := range in {
		conn.Close()
	}
}

func (proxy ProxyInstance) start() {
	flag.Parse()

	addr, err := net.ResolveTCPAddr("tcp", buildListenAddress(proxy))
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	pending, complete := make(chan *net.TCPConn), make(chan *net.TCPConn)

	for i := 0; i < 5; i++ {
		go proxy.handleConn(pending, complete)
	}
	go proxy.closeConn(complete)

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			panic(err)
		}
		pending <- conn
		logproxy.Debugf("Got shit\n")
	}
}

func buildListenAddress(configuration ProxyInstance) string {
	return configuration.ListenAddress + ":" + strconv.Itoa(configuration.ListenPort)
}

func buildConnectAddress(configuration ProxyInstance) string {
	return configuration.ConnectAddress + ":" + strconv.Itoa(configuration.ConnectPort)
}

func main_test() {
	proxy := ProxyInstance{}
	proxy.ListenAddress = "192.168.0.109"
	proxy.ListenPort = 6275
	proxy.ConnectAddress = "162.17.206.195"
	proxy.ConnectPort = 80
	proxy.start()

}