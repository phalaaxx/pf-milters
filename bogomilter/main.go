/* bogomilter is a milter service for postfix */
package main

import (
	"flag"
	"fmt"
	"github.com/phalaaxx/milter"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

/* global variables */
var BogoBin string
var BogoDir string
var LocalHold bool

/* BogoMilter object */
type BogoMilter struct {
	milter.Milter
	from   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

/* Header parses message headers one by one */
func (b BogoMilter) Header(name, value string, m *milter.Modifier) (milter.Response, error) {
	// check if bogofilter has been run on the message already
	if name == "X-Bogosity" {
		// X-Bogosity header is present, accept immediately
		return milter.RespAccept, nil
	}
	return milter.RespContinue, nil
}

/* MailFrom is called on envelope from address */
func (b *BogoMilter) MailFrom(from string, m *milter.Modifier) (milter.Response, error) {
	// save from address for later reference
	b.from = from
	return milter.RespContinue, nil
}

/* Headers is called after the last of message headers */
func (b *BogoMilter) Headers(headers textproto.MIMEHeader, m *milter.Modifier) (milter.Response, error) {
	var err error
	// end of headers, start bogofilter pipe
	b.cmd = exec.Command(BogoBin, "-v", "-d", BogoDir)
	// get bogofilter stdin
	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// get bogofilter stdout
	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// start command
	if err = b.cmd.Start(); err != nil {
		return nil, err
	}
	// print headers to stdin
	for k, vl := range headers {
		for _, v := range vl {
			if _, err := fmt.Fprintf(b.stdin, "%s: %s\n", k, v); err != nil {
				return nil, err
			}
		}
	}
	if _, err := fmt.Fprintf(b.stdin, "\n"); err != nil {
		return nil, err
	}

	return milter.RespContinue, nil
}

// accept body chunk
func (b BogoMilter) BodyChunk(chunk []byte, m *milter.Modifier) (milter.Response, error) {
	// send chunk to bogofilter stdin
	if _, err := b.stdin.Write(chunk); err != nil {
		return nil, err
	}
	return milter.RespContinue, nil
}

/* Body is called when email message body has been sent */
func (b *BogoMilter) Body(m *milter.Modifier) (milter.Response, error) {
	// close process stdin and read its output
	if err := b.stdin.Close(); err != nil {
		return nil, err
	}
	// get bogofilter output
	output, err := ioutil.ReadAll(b.stdout)
	if err != nil {
		return nil, err
	}
	// wait for process to terminate
	if err := b.cmd.Wait(); err != nil {
		// no easy way to get exit code
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				// exit code is in status.ExitStatus()
				if status.ExitStatus() == 3 {
					// exit code 3 indicates error condition
					return nil, err
				}
			}
		}
	}
	// add X-Bogosity header
	header := string(output)
	if strings.HasPrefix(header, "X-Bogosity") {
		if m.AddHeader("X-Bogosity", header[12:len(header)-1]); err != nil {
			return nil, err
		}
		// log spam senders
		if strings.HasPrefix(header, "X-Bogosity: Spam") {
			fmt.Printf("detected spam from %s\n", b.from)
		}
		// put locally originating spam into quarantine
		if LocalHold && len(m.Headers.Get("Received")) == 0 {
			if strings.HasPrefix(header, "X-Bogosity: Spam") {
				fmt.Printf("quarantine mail from %s\n", b.from)
				m.Quarantine("local spam")
				// TODO: notify administrator
			}
		}
	}
	return milter.RespAccept, nil
}

/* NewObject creates new BogoMilter instance */
func RunServer(socket net.Listener) {
	// declare milter init function
	init := func() (milter.Milter, uint32, uint32) {
		return &BogoMilter{},
			milter.OptAddHeader | milter.OptChangeHeader,
			milter.OptNoConnect | milter.OptNoHelo | milter.OptNoRcptTo
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
		"/var/spool/postfix/milter/bogo.sock",
		"Bind to address or unix domain socket")
	flag.StringVar(&BogoBin,
		"bin",
		"/usr/bin/bogofilter",
		"Full path to bogofilter binary")
	flag.StringVar(&BogoDir,
		"db",
		"/var/cache/filter",
		"Path to bogofilter database")
	flag.BoolVar(&LocalHold,
		"localhold",
		false,
		"Put outgoing spam into quarantine")
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
