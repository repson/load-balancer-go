package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sync/atomic"
)

var requestCount atomic.Uint64

func main() {
	// Start two TCP echo servers on different ports
	go startServer(4001, "TCP-Server-1")
	go startServer(4002, "TCP-Server-2")

	// Block forever
	select {}
}

func startServer(port int, name string) {
	var localCount atomic.Uint64

	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("Error starting %s: %v", name, err)
		os.Exit(1)
	}
	defer listener.Close()

	log.Printf("Starting %s on localhost%s", name, addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[%s] Error accepting connection: %v", name, err)
			continue
		}

		// Handle each connection in a goroutine
		go handleConnection(conn, name, &localCount)
	}
}

func handleConnection(conn net.Conn, serverName string, localCount *atomic.Uint64) {
	defer conn.Close()

	local := localCount.Add(1)
	global := requestCount.Add(1)
	clientAddr := conn.RemoteAddr().String()

	log.Printf("[%s] Connection #%d from %s", serverName, local, clientAddr)

	// Send welcome message
	welcome := fmt.Sprintf("Connected to %s\n", serverName)
	welcome += fmt.Sprintf("Local connections: %d\n", local)
	welcome += fmt.Sprintf("Total connections: %d\n", global)
	welcome += "Echo mode: Type a message and press Enter\n"
	welcome += "Type 'quit' to close connection\n\n"
	conn.Write([]byte(welcome))

	// Read and echo back messages
	scanner := bufio.NewScanner(conn)
	messageCount := 0

	for scanner.Scan() {
		message := scanner.Text()
		messageCount++

		if message == "quit" {
			log.Printf("[%s] Client %s disconnected (sent quit)", serverName, clientAddr)
			conn.Write([]byte("Goodbye!\n"))
			break
		}

		response := fmt.Sprintf("[%s] Echo #%d: %s\n", serverName, messageCount, message)
		conn.Write([]byte(response))
		log.Printf("[%s] Echoed message #%d from %s: %s", serverName, messageCount, clientAddr, message)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[%s] Connection error: %v", serverName, err)
	}

	log.Printf("[%s] Connection from %s closed (%d messages)", serverName, clientAddr, messageCount)
}
