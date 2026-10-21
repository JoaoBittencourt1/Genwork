package mail

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
)

// fakeSMTPServer speaks just enough SMTP to accept one message per connection.
type fakeSMTPServer struct {
	listener      net.Listener
	advertiseAuth bool
	mutex         sync.Mutex
	messages      []string
	authLines     []string
}

func startFakeSMTPServer(t *testing.T, advertiseAuth bool) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &fakeSMTPServer{listener: listener, advertiseAuth: advertiseAuth}
	go server.acceptConnections()
	t.Cleanup(func() { listener.Close() })
	return server
}

func (server *fakeSMTPServer) port() int {
	return server.listener.Addr().(*net.TCPAddr).Port
}

func (server *fakeSMTPServer) receivedMessages() []string {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return append([]string(nil), server.messages...)
}

func (server *fakeSMTPServer) receivedAuth() []string {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return append([]string(nil), server.authLines...)
}

func (server *fakeSMTPServer) acceptConnections() {
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		go server.serve(connection)
	}
}

func (server *fakeSMTPServer) serve(connection net.Conn) {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	reply := func(line string) { _, _ = connection.Write([]byte(line + "\r\n")) }
	reply("220 fake.smtp ready")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(command, "EHLO"):
			if server.advertiseAuth {
				reply("250-fake.smtp")
				reply("250 AUTH PLAIN")
			} else {
				reply("250 fake.smtp")
			}
		case strings.HasPrefix(command, "HELO"):
			reply("250 fake.smtp")
		case strings.HasPrefix(command, "AUTH"):
			server.mutex.Lock()
			server.authLines = append(server.authLines, strings.TrimSpace(line))
			server.mutex.Unlock()
			reply("235 authenticated")
		case strings.HasPrefix(command, "MAIL"), strings.HasPrefix(command, "RCPT"), strings.HasPrefix(command, "RSET"), strings.HasPrefix(command, "NOOP"):
			reply("250 ok")
		case strings.HasPrefix(command, "DATA"):
			reply("354 end with <CRLF>.<CRLF>")
			var body strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				body.WriteString(dataLine)
			}
			server.mutex.Lock()
			server.messages = append(server.messages, body.String())
			server.mutex.Unlock()
			reply("250 queued")
		case strings.HasPrefix(command, "QUIT"):
			reply("221 bye")
			return
		default:
			reply("502 not implemented")
		}
	}
}
