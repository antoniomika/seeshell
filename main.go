package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"

	"net"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type tcpConnData struct {
	Conn   net.Conn
	Buffer []byte
}

type wsConnData struct {
	Conn        *websocket.Conn
	Initialized bool
}

var (
	secretPath   = flag.String("secretpath", "", "The path to look for to print session ids, empty string to disable")
	debug        = flag.Bool("debug", false, "Whether or not to print debug info")
	httpDomain   = flag.String("httpdomain", "localhost", "The domain for the service to be outputted")
	httpsEnabled = flag.Bool("httpsenabled", false, "Whether HTTPS is enabled (reverse proxy)")
	httpPort     = flag.Int("httpport", 8080, "What port to display")
	httpAddr     = flag.String("httpaddr", "localhost:8080", "HTTP/WS service address")
	tcpAddr      = flag.String("tcpaddr", "localhost:8081", "TCP service address")
	tcpTransAddr = flag.String("tcptransaddr", "localhost:8082", "TCP transparent proxy service address")
	upgrader     = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	wsConns  = &sync.Map{}
	tcpConns = &sync.Map{}
)

func main() {
	flag.Parse()

	if *debug {
		go func() {
			for {
				log.Println(runtime.NumGoroutine())

				log.Println("TCP CONNS")
				tcpConns.Range(func(key, val interface{}) bool {
					log.Println(key)
					return true
				})

				log.Println("WS CONNS1")
				wsConns.Range(func(key, val interface{}) bool {
					log.Println(key)
					val2 := val.(*sync.Map)

					log.Println("WS CONNS2")
					val2.Range(func(key, val interface{}) bool {
						log.Println(key)
						return true
					})
					return true
				})

				time.Sleep(10 * time.Second)
			}
		}()
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/:id", func(c *gin.Context) {
		switch id := c.Param("id"); id {
		case *secretPath:
			conns := []string{}
			tcpConns.Range(func(key, val interface{}) bool {
				addr := key.(string)
				conns = append(conns, addr)
				return true
			})

			c.HTML(http.StatusOK, "listpage.html", conns)
		case "":
			fallthrough
		default:
			c.HTML(http.StatusOK, "index.html", c)
		}
	})

	r.GET("/:id/ws", wsHandler)

	go startTCP(*tcpAddr, false)
	go startTCP(*tcpTransAddr, true)
	r.Run(*httpAddr)
}

func wsHandler(c *gin.Context) {
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)

	connData := &wsConnData{
		Conn: wsConn,
	}

	pathKey := c.Param("id")
	keyPress := false

	if strings.Contains(pathKey, "show") {
		pathKey = strings.ReplaceAll(pathKey, "show", "")
		keyPress = true
	}

	if err != nil {
		return
	}

	conns, _ := wsConns.LoadOrStore(pathKey, &sync.Map{})
	addressedWSConns := conns.(*sync.Map)

	addressedWSConns.Store(wsConn.RemoteAddr().String(), connData)

	defer func() {
		addressedWSConns.Delete(wsConn.RemoteAddr().String())

		count := 0
		addressedWSConns.Range(func(key, val interface{}) bool {
			count++
			return true
		})

		if count == 0 {
			wsConns.Delete(pathKey)
		}
		wsConn.Close()
	}()

	if tcpClientInterface, ok := tcpConns.Load(pathKey); ok {
		tcpClient := tcpClientInterface.(*tcpConnData)

		wsConn.WriteMessage(websocket.TextMessage, tcpClient.Buffer)
		connData.Initialized = true

		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				break
			}

			if keyPress {
				wsConn.WriteMessage(websocket.TextMessage, data)
			}

			tcpClient.Conn.Write(data)
		}
	}
}

func startTCP(addr string, transparent bool) {
	socket, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()

	for {
		conn, err := socket.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go handleTCP(conn, transparent)
	}
}

func handleTCP(conn net.Conn, transparent bool) {
	conn.SetReadDeadline(time.Now())
	reader := bufio.NewReader(conn)

	setConn := &tcpConnData{
		Conn: conn,
	}

	tcpConns.Store(conn.RemoteAddr().String(), setConn)

	defer func() {
		tcpConns.Delete(conn.RemoteAddr().String())
		conn.Close()
	}()

	if !transparent {
		scheme := "http"
		if *httpsEnabled {
			scheme = "https"
		}

		conn.Write([]byte(fmt.Sprintf("Terminal output redirected to %s://%s:%d/%s\r\n", scheme, *httpDomain, *httpPort, conn.RemoteAddr().String())))
	}

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Millisecond))

		data, err := reader.Peek(1)

		neterr, ok := err.(net.Error)
		if err != nil {
			if ok && neterr.Timeout() {
				continue
			}
			break
		}

		if len(data) > 0 {
			realData := make([]byte, 256)
			n, err := reader.Read(realData)

			if n == 0 && err != nil {
				log.Println(err)
				break
			}
			realData = bytes.ReplaceAll(realData, []byte{'\r', '\n'}, []byte{'\n'})
			realData = bytes.ReplaceAll(realData, []byte{'\n'}, []byte{'\r', '\n'})

			setConn.Buffer = append(setConn.Buffer, realData...)

			if addressedWSConnsInterface, ok := wsConns.Load(conn.RemoteAddr().String()); ok {
				addressedWSConns := addressedWSConnsInterface.(*sync.Map)

				addressedWSConns.Range(func(key, value interface{}) bool {
					wsClient := value.(*wsConnData)
					if wsClient.Initialized {
						wsClient.Conn.WriteMessage(websocket.TextMessage, realData)
					}
					return true
				})
			}
		}
	}
}
