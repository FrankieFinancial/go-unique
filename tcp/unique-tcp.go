//
// main - a TCP server to listen for unique commands and respond with the calculated value.
//
// @author darryl.west <darryl.west@raincitysoftware.com>
// @created 2017-09-25 08:27:43
//

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"
	"github.com/FrankieFinancial/go-unique/unique"
)

// Server - port, timeout, etc
type Server struct {
	Port        int
	IdleTimeout time.Duration
}

// Client - the id, timeout, request count, etc
type Client struct {
	id          string
	IdleTimeout time.Duration
	Requests    int
	buffer      [64]byte
}

// the default bytes calculation
func bytes() string {
	b, _ := unique.RandomBytes(24)
	return fmt.Sprintf("%x", b)
}

// CommandMap a map of recognized commands that may be send from clinets; if not in the list then 'error' is returned
type CommandMap map[string]func() string

var commands = CommandMap{
	"ping":    func() string { return "pong" },
	"noop":    func() string { return "ok" },
	"version": unique.Version,
	"uuid":    unique.CreateUUID,
	"ulid":    unique.CreateULID,
	"guid":    unique.CreateGUID,
	"tsid":    unique.CreateTSID,
	"txid":    unique.CreateTXID,
	"cuid":    unique.CreateCUID,
	"xuid":    unique.CreateXUID,
	"bytes":   bytes,
}

// this could be implemented with a Reader/Writer/Scanner and implement SetDeadline, but this is a bit lighter weight...
func (cli *Client) readRequest(conn net.Conn) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cli.IdleTimeout)
	defer cancel()

	var (
		response string
		err      error
		ccount   int
	)

	complete := make(chan bool, 1)

	go func() {
		buf := cli.buffer[:]
		ccount, err = conn.Read(buf)
		// fmt.Println("ccount:", ccount)
		if err == nil && ccount > 0 {
			response = strings.TrimSpace(string(buf[:ccount]))
		}

		cli.Requests++

		complete <- true
	}()

	select {
	case <-ctx.Done():
		return response, ctx.Err()
	case <-complete:
		return response, err
	}
}

func (cli Client) handleClient(conn net.Conn) {
	defer func() {
		fmt.Printf("closing connection from %v, id: %s\n", conn.RemoteAddr(), cli.id)
		conn.Close()
	}()

	for {
		request, err := cli.readRequest(conn)
		if err != nil {
			break
		}

		var response string

		// parse the request
		if cmd, ok := commands[request]; ok {
			response = cmd()
		} else {
			response = "error"
		}

		// fmt.Printf("client: %s request: %s, response: %s\n", cli.id, request, response)

		fmt.Fprintf(conn, "%s\n\r", response)
	}
}

func main() {
	svr := parseArgs()
	if svr == nil {
		return
	}

	// start the server
	host := fmt.Sprintf("0.0.0.0:%d", svr.Port)
	ss, err := net.Listen("tcp", host)
	if err != nil {
		fmt.Printf("error opening host %s...\n", host)
		os.Exit(1)
	}

	fmt.Printf("listening on host: %s\n", host)

	defer ss.Close()
	for {
		conn, err := ss.Accept()
		if err != nil {
			fmt.Println("Accept error: ", err.Error())
			continue
		}

		// create a client struct and add to the list
		client := Client{id: unique.CreateTXID(), IdleTimeout: svr.IdleTimeout}
		go client.handleClient(conn)
	}
}

func parseArgs() *Server {
	tos := 300
	svr := Server{Port: 3001, IdleTimeout: time.Duration(tos) * time.Second}

	vers := flag.Bool("version", false, "show the version and exit")
	port := flag.Int("port", svr.Port, "set the listening port")
	timeout := flag.Int("timeout", tos, "set the client's idle timeout in seconds")

	flag.Parse()

	// show the version
	fmt.Printf("%s Version: %s\n", path.Base(os.Args[0]), unique.Version())
	if *vers == true {
		return nil
	}

	svr.Port = *port
	svr.IdleTimeout = time.Duration(*timeout) * time.Second

	// show the port and idle timeout
	return &svr
}
