package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
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
	debug        = flag.Bool("debug", false, "Whether or not to print debug info")
	httpDomain   = flag.String("httpdomain", "localhost", "The domain for the service to be outputted")
	httpsEnabled = flag.Bool("httpsenabled", false, "Whether HTTPS is enabled (reverse proxy)")
	httpPort     = flag.Int("httpport", 8080, "What port to display")
	httpAddr     = flag.String("httpaddr", "localhost:8080", "HTTP/WS service address")
	tcpAddr      = flag.String("tcpaddr", "localhost:8081", "TCP service address")
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

				time.Sleep(2 * time.Second)
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
		c.HTML(http.StatusOK, "index.html", c.Params)
	})

	r.GET("/:id/ws", wsHandler)

	go startTCP()
	r.Run(*httpAddr)
}

func wsHandler(c *gin.Context) {
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)

	connData := &wsConnData{
		Conn: wsConn,
	}

	pathKey := c.Param("id")

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

		w, err := wsConn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}

		w.Write(tcpClient.Buffer)
		connData.Initialized = true

		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				break
			}

			tcpClient.Conn.Write(data)
		}
	}
}

func startTCP() {
	socket, err := net.Listen("tcp", *tcpAddr)
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

		go handleTCP(conn)
	}
}

func handleTCP(conn net.Conn) {
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

	scheme := "http"
	if *httpsEnabled {
		scheme = "https"
	}

	conn.Write([]byte(fmt.Sprintf("Terminal output redirected to %s://%s:%d/%s\r\n", scheme, *httpDomain, *httpPort, conn.RemoteAddr().String())))

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
			realData, err := ioutil.ReadAll(reader)

			if len(realData) == 0 && err != nil {
				log.Println(err)
				break
			}

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
