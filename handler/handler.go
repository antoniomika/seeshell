// Package handler implements the main seeshell handler
package handler

import (
	"bufio"
	"bytes"
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
	"github.com/spf13/viper"
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
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	wsConns  = &sync.Map{}
	tcpConns = &sync.Map{}
)

// Handle executes the main handler.
func Handle() {
	if viper.GetBool("debug") {
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
	r.Static("/static", "static/")

	if viper.GetString("secret-path") != "" {
		r.GET(fmt.Sprintf("/%s", viper.GetString("secret-path")), func(c *gin.Context) {
			conns := []string{}
			tcpConns.Range(func(key, val interface{}) bool {
				addr := key.(string)
				conns = append(conns, addr)
				return true
			})

			c.HTML(http.StatusOK, "listpage.html", conns)
		})
	}

	r.GET("/socket/:id", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", c)
	})

	r.GET("/socket/:id/ws", wsHandler)

	go startTCP(viper.GetString("tcp-address"), false)
	go startTCP(viper.GetString("tcp-transparent-address"), true)

	log.Fatalln(r.Run(viper.GetString("http-address")))
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

		err := wsConn.WriteMessage(websocket.BinaryMessage, tcpClient.Buffer)
		if err != nil {
			log.Println("error writing message to socket:", err)
		}

		connData.Initialized = true

		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				break
			}

			if keyPress {
				err := wsConn.WriteMessage(websocket.BinaryMessage, data)
				if err != nil {
					log.Println("error writing message to socket:", err)
				}
			}

			_, err = tcpClient.Conn.Write(data)
			if err != nil {
				log.Println("error writing message to socket:", err)
			}
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
	err := conn.SetReadDeadline(time.Now())
	if err != nil {
		log.Println("error setting read deadline:", err)
	}

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
		if viper.GetBool("https-enabled") {
			scheme = "https"
		}

		_, err := conn.Write([]byte(fmt.Sprintf("Terminal output redirected to %s://%s:%d/socket/%s\r\n", scheme, viper.GetString("http-domain"), viper.GetInt("http-port"), conn.RemoteAddr().String())))
		if err != nil {
			log.Println("error writing message to socket:", err)
		}
	}

	for {
		err := conn.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
		if err != nil {
			log.Println("error setting read deadline:", err)
		}

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
						err := wsClient.Conn.WriteMessage(websocket.BinaryMessage, realData)
						if err != nil {
							log.Println("error writing message to socket:", err)
						}
					}
					return true
				})
			}
		}
	}
}
