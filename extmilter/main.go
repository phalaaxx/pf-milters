package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/phalaaxx/milter"
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
)

/* ExtMilter object */
type ExtMilter struct {
	milter.Milter
	multipart bool
	message   *bytes.Buffer
}

/* handle headers one by one */
func (e *ExtMilter) Header(name, value string, m *milter.Modifier) (milter.Response, error) {
	// if message has multiple parts set processing flag to true
	if name == "Content-Type" && strings.HasPrefix(value, "multipart/") {
		e.multipart = true
	}
	return milter.RespContinue, nil
}

/* at end of headers initialize message buffer and add headers to it */
func (e *ExtMilter) Headers(headers textproto.MIMEHeader, m *milter.Modifier) (milter.Response, error) {
	// return accept if not a multipart message
	if !e.multipart {
		return milter.RespAccept, nil
	}
	// prepare message buffer
	e.message = new(bytes.Buffer)
	// print headers to message buffer
	for k, vl := range headers {
		for _, v := range vl {
			if _, err := fmt.Fprintf(e.message, "%s: %s\n", k, v); err != nil {
				return nil, err
			}
		}
	}
	if _, err := fmt.Fprintf(e.message, "\n"); err != nil {
		return nil, err
	}
	// continue with milter processing
	return milter.RespContinue, nil
}

// accept body chunk
func (e *ExtMilter) BodyChunk(chunk []byte, m *milter.Modifier) (milter.Response, error) {
	// save chunk to buffer
	if _, err := e.message.Write(chunk); err != nil {
		return nil, err
	}
	return milter.RespContinue, nil
}

/* Body is called when email message body has been sent */
func (e *ExtMilter) Body(m *milter.Modifier) (milter.Response, error) {
	// prepare buffer
	buffer := bytes.NewReader(e.message.Bytes())
	// parse email message and get accept flag
	if err := ParseEmailMessage(buffer); err != nil {
		if err == EPayloadNotAllowed {
			// return custom response message
			return milter.NewResponseStr('y', err.Error()), nil
		}
		return nil, err
	}
	// accept message by default
	return milter.RespAccept, nil
}

/* NewObject creates new BogoMilter instance */
func RunServer(socket net.Listener) {
	// declare milter init function
	init := func() (milter.Milter, uint32, uint32) {
		return &ExtMilter{},
			milter.OptAddHeader | milter.OptChangeHeader,
			milter.OptNoConnect | milter.OptNoHelo | milter.OptNoMailFrom | milter.OptNoRcptTo
	}
	// start server
	if err := milter.RunServer(socket, init); err != nil {
		log.Fatal(err)
	}
}

/* main program */
func main() {
	// parse commandline arguments
	var protocol, address string
	flag.StringVar(&protocol,
		"proto",
		"unix",
		"Protocol family (unix or tcp)")
	flag.StringVar(&address,
		"addr",
		"/var/spool/postfix/milters/ext.sock",
		"Bind to address or unix domain socket")
	flag.Parse()

	// make sure the specified protocol is either unix or tcp
	if protocol != "unix" && protocol != "tcp" {
		log.Fatal("invalid protocol name")
	}

	// make sure socket does not exist
	if protocol == "unix" {
		// ignore os.Remove errors
		os.Remove(address)
	}

	// bind to listening address
	socket, err := net.Listen(protocol, address)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()

	if protocol == "unix" {
		// set mode 0660 for unix domain sockets
		if err := os.Chmod(address, 0660); err != nil {
			log.Fatal(err)
		}
		// remove socket on exit
		defer os.Remove(address)
	}

	// run server
	go RunServer(socket)

	// sleep forever
	select {}
}
