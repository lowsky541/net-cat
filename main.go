package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

///////////////////////////////////////////////////////////
// Config
///////////////////////////////////////////////////////////

const (
	NAME = "Kevin"
	HOST = "0.0.0.0"
	PORT = 8989
)

///////////////////////////////////////////////////////////
// Utilities
///////////////////////////////////////////////////////////

func stringIsEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

func ParseCommand(s string) []string {
	runes := []rune(s)
	if len(runes) > 0 && runes[0] == '/' {
		s = string(runes[1:])
		//a := strings.Split(s, " ")
		a := strings.FieldsFunc(s, func(r rune) bool {
			return r == ' '
		})
		return a
	}
	return nil
}

///////////////////////////////////////////////////////////
// Structures/Enums
///////////////////////////////////////////////////////////

type Client struct {
	id       int
	username string
	conn     net.Conn
	kicks    int
}

type Message struct {
	clientId int
	username string
	message  string
	when     time.Time
}

///////////////////////////////////////////////////////////
// Main code
///////////////////////////////////////////////////////////

var (
	lastClientId int
	clients      map[int]*Client = map[int]*Client{}
	messages     []Message

	messagesMutex sync.Mutex
	clientsMutex  sync.Mutex
)

func InService(name string) bool {
	for _, client := range clients {
		if client.username == name {
			return true
		}
	}
	return false
}

func BroadcastAllFormatted(senderId int, format string, args ...interface{}) {
	for _, client := range clients {
		if client.username != "" {
			fmt.Fprintf(client.conn, format, args...)
			if client.id != senderId {
				now := time.Now().Format("2006-01-02 15:04:05")
				fmt.Fprintf(client.conn, "[%s][%s]: ", now, client.username)
			}
		}
	}
}

func BroadcastFormatted(senderId int, format string, args ...interface{}) {
	for _, client := range clients {
		if client.id != senderId && client.username != "" {
			fmt.Fprintf(client.conn, format, args...)
		}
	}
}

func BroadcastChangedName(self *Client, oldUsername string) {
	for _, client := range clients {
		if client.id != self.id && client.username != "" {
			fmt.Fprintf(client.conn, "\nThe magnificient %s became %s\n", oldUsername, self.username)
			now := time.Now().Format("2006-01-02 15:04:05")
			fmt.Fprintf(client.conn, "[%s][%s]: ", now, client.username)
		}
	}
}

func BroadcastJoin(self *Client) {
	fmt.Printf("Client connection: %d/%s\n", self.id, self.username)
	for _, client := range clients {
		if client.id != self.id && client.username != "" {
			fmt.Fprintf(client.conn, "\nThe magnificient %s joined the room!!!\n", self.username)
			now := time.Now().Format("2006-01-02 15:04:05")
			fmt.Fprintf(client.conn, "[%s][%s]: ", now, client.username)
		}
	}
}

func BroadcastUnjoin(self *Client) {
	fmt.Printf("Client disconnection: %d/%s\n", self.id, self.username)
	for _, client := range clients {
		if client.id != self.id && client.username != "" {
			fmt.Fprintf(client.conn, "\nThe magnificient %s leaved the room...\n", self.username)
			now := time.Now().Format("2006-01-02 15:04:05")
			fmt.Fprintf(client.conn, "[%s][%s]: ", now, client.username)
		}
	}
}

func (client *Client) handleRequestWithCleanup(conn net.Conn) {
	client.handleRequest(conn)
	delete(clients, client.id)
	if client.username != "" {
		BroadcastUnjoin(client)
	}
}

func FindClient(name string) *Client {
	for _, c := range clients {
		if c.username == name {
			return c
		}
	}
	return nil
}

func (myself *Client) handleRequest(conn net.Conn) {
	reader := bufio.NewReader(conn)

	if err := writeMascot(conn, "./mascots"); err != nil {
		fmt.Printf("failure writing mascot: %s\n", err.Error())
	}

	fmt.Fprintf(conn, "Hey %s! What's your name ? ", NAME)
	rawUsername, err := reader.ReadString('\n')
	if err != nil || stringIsEmpty(rawUsername) || len(rawUsername) > 20 {
		fmt.Fprintf(conn, "Usernames must not exceed 20 characters (excluded)\n")
		conn.Close()
		return
	}

	username := strings.ReplaceAll(rawUsername, "\n", "")
	if InService(username) {
		fmt.Fprintf(conn, "This name is already in use; please retry with another\n")
		conn.Close()
		return
	}

	myself.username = username

	fmt.Fprintf(conn, "Welcome to the chat, %s\n", NAME)
	BroadcastJoin(myself)

	for _, m := range messages {
		when := m.when.Format("2006-01-02 15:04:05")
		fmt.Fprintf(conn, "[%s][%s]: %s\n", when, m.username, m.message)
	}

	for {
		now := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(conn, "[%s][%s]: ", now, myself.username)

		rawMessage, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return
		}

		message := strings.ReplaceAll(rawMessage, "\n", "")
		if stringIsEmpty(message) {
			fmt.Fprintf(conn, "Empty messages are not broadcasted\n")
		} else {
			command := ParseCommand(message)
			if len(command) == 1 && command[0] == "help" {
				fmt.Fprintf(conn, "/kick\n")
				fmt.Fprintf(conn, "/name <name>\n")
				fmt.Fprintf(conn, "/overdose\n")
				fmt.Fprintf(conn, "/fuckthemall\n")
				continue
			} else if len(command) == 2 && command[0] == "name" {
				oldUsername := myself.username
				newUsername := command[1]
				if !InService(newUsername) {
					myself.username = newUsername
					BroadcastChangedName(myself, oldUsername)
				}
				continue
			} else if len(command) == 2 && command[0] == "kick" {
				name := command[1]
				fucker := FindClient(name)
				if fucker == nil {
					fmt.Fprintf(conn, "Who the hell do you want to kick?...\n")
				} else if fucker.id == myself.id {
					BroadcastFormatted(myself.id, "%s wants to kick themselves...\n", myself.username)
				} else {
					fucker.kicks++
					if fucker.kicks == 3 {

						// Broadcast kick
						for _, client := range clients {
							if client.username != "" {
								newline := ""
								if client.id == fucker.id {
									newline = "\n"
								}

								fmt.Fprintf(client.conn, "Bye bye, %s%s", fucker.username, newline)
								if client.id != fucker.id && client.id != myself.id {
									now := time.Now().Format("2006-01-02 15:04:05")
									fmt.Fprintf(client.conn, "[%s][%s]: ", now, client.username)
								}
							}
						}
						// End broadcast

						fucker.conn.Close()
						continue
					}

					if fucker.kicks == 1 {
						BroadcastFormatted(myself.id, "\n%s wants to kick %s; \"/kick %s\" to contribute to the room!\n", myself.username, name, name)
					}
					BroadcastAllFormatted(myself.id, "%s is now at kick count %d\n", name, fucker.kicks)
				}
				continue
			} else if len(command) == 1 && command[0] == "overdose" {
				fr, _ := os.OpenFile("/dev/random", os.O_RDONLY, 0666)
				defer fr.Close()
				io.Copy(conn, fr)
			} else if len(command) == 1 && command[0] == "fuckthemall" {
				for _, c := range clients {
					if c.id != myself.id && c.username != "" {
						go func(client *Client) {
							fr, _ := os.OpenFile("/dev/random", os.O_RDONLY, 0666)
							defer fr.Close()
							// If the client disconnect, io.Copy will die and the function will end
							io.Copy(client.conn, fr)
						}(c)
					}
				}
			}

			when := time.Now()

			messagesMutex.Lock()
			messages = append(messages, Message{
				clientId: myself.id,
				username: myself.username,
				message:  message,
				when:     when,
			})
			messagesMutex.Unlock()

			for _, c := range clients {
				if c.id != myself.id && c.username != "" {
					fmt.Fprintf(c.conn, "\n[%s][%s]: %s\n", when.Format("2006-01-02 15:04:05"), myself.username, message)
					now := time.Now().Format("2006-01-02 15:04:05")
					fmt.Fprintf(c.conn, "[%s][%s]: ", now, c.username)
				}
			}
		}
	}
}

func usage() {
	fmt.Printf("usage:  tcp-chat port\n")
	fmt.Printf("  port  The port to bind to (default: %d)\n", PORT)
}

func main() {
	port := PORT
	args := os.Args[1:]

	var atoiErr error = nil

	if len(args) == 1 {
		arg := args[0]
		port, atoiErr = strconv.Atoi(arg)
	}

	if len(args) > 1 || atoiErr != nil {
		usage()
		return
	}

	addr := fmt.Sprintf("%s:%d", HOST, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	log.Printf("Listening on %s...", addr)

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("cannot accept: %s", err.Error())
			continue
		}

		clientsMutex.Lock()
		lastClientId = lastClientId + 1
		client := &Client{id: lastClientId, conn: conn}
		clients[lastClientId] = client
		clientsMutex.Unlock()

		go client.handleRequestWithCleanup(conn)
	}
}
