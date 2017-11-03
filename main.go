package main

import (
	"flag"
	"log"
	"net/http"
	"text/template"

	"github.com/fatih/color"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"net"
	"time"
	"sync"
	"bufio"
)

var httpAddr = flag.String("http-addr", "localhost:8080", "HTTP/WS service address")
var tcpAddr = flag.String("tcp-add", "localhost:8081", "TCP service address")

var indexTemplate = template.Must(template.ParseFiles("templates/index.html"))

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var webSocketClients = make(map[string][]*websocket.Conn)
var tcpClients = make(map[string]net.Conn)

var mutex = &sync.Mutex{}

func logHTTP(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		color.Set(color.FgYellow)
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		color.Unset()

		handler.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	r := mux.NewRouter()

	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/ws/{id}", wsHandler)

	http.Handle("/", r)

	color.Set(color.FgGreen)
	log.Println("Running HTTP and WS server on:", *httpAddr)
	color.Unset()

	color.Set(color.FgRed)
	go http.ListenAndServe(*httpAddr, logHTTP(http.DefaultServeMux))
	color.Unset()

	color.Set(color.FgGreen)
	log.Println("Running TCP server on:", *tcpAddr)
	color.Unset()

	startTCP()
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	indexTemplate.Execute(w, r.Host)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pathKey := vars["id"]

	c, err := upgrader.Upgrade(w, r, nil)

	color.Set(color.FgBlue)
	log.Println("New WebSocket Connection From:", r.RemoteAddr)
	log.Println("Path:", pathKey)
	color.Unset()

	if err != nil {
		color.Set(color.FgRed)
		log.Println("Upgrade error:", err)
		color.Unset()
		return
	}

	mutex.Lock()

	if _, ok := webSocketClients[pathKey]; ok {
		webSocketClients[pathKey] = append(webSocketClients[pathKey], c)
	} else {
		webSocketClients[pathKey] = []*websocket.Conn{c}
	}

	mutex.Unlock()

	for {
		_, _, err := c.ReadMessage()

		if err != nil {
			color.Set(color.FgRed)
			log.Println("wsReader error:", err)
			color.Unset()

			break
		}

		// This will redirect input from the web based terminal to the client (netcat).
		// If netcat bidirectionally opened stdin and stdout, you could control your process with that.
		/*if val, ok := tcpClients[pathKey]; ok {
			writer := bufio.NewWriter(val)

			writer.Write(data)
			writer.Flush()
		}*/
	}

	defer func() {
		c.Close()

		color.Set(color.FgMagenta)
		log.Println("Closed WebSocket Connection From:", r.RemoteAddr)
		color.Unset()

		mutex.Lock()

		if _, ok := webSocketClients[pathKey]; ok {
			newclients := []*websocket.Conn{}
			for _, varclient := range webSocketClients[pathKey] {
				if c != varclient {
					newclients = append(newclients, varclient)
				}
			}
			webSocketClients[pathKey] = newclients
		} else {
			webSocketClients[pathKey] = []*websocket.Conn{}
		}

		mutex.Unlock()
	}()
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

		color.Set(color.FgBlue)
		log.Println("New TCPServer Connection From:", conn.RemoteAddr().String())
		color.Unset()

		go handleTCP(conn)
	}
}

func handleTCP(conn net.Conn) {
	tcpClients[conn.RemoteAddr().String()] = conn

	conn.SetReadDeadline(time.Now())
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	writer.Write([]byte("Access the RTV at http://" + *httpAddr + "/#" + conn.RemoteAddr().String() + "\n"))
	writer.Flush()

	for {
		zero := make([]byte, 0)

		if _, err := conn.Read(zero); err != nil {
			log.Println("Foobbar", err)
			break
		} else {
			conn.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
		}

		mutex.Lock()

		if data, _ := reader.Peek(1); len(data) > 0 {
			if val, ok := webSocketClients[conn.RemoteAddr().String()]; ok {
				data, _, err := reader.ReadLine()
				if err != nil {
					color.Set(color.FgRed)
					log.Println("TCPReader error:", err)
					color.Unset()
				}

				for _, wsClient := range val {
					wsWriter, err := wsClient.NextWriter(websocket.TextMessage)
					if err != nil {
						color.Set(color.FgRed)
						log.Println("wsWriter error:", err)
						color.Unset()
					} else {
						wsWriter.Write(data)
						wsWriter.Close()
					}
				}
			}
		}

		mutex.Unlock()
	}

	defer func() {
		conn.Close()

		color.Set(color.FgMagenta)
		log.Println("Closed TCPServer Connection From:", conn.RemoteAddr().String())
		color.Unset()
	}()
}