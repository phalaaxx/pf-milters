package main

import (
	"encoding/base64"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"path/filepath"
	"strings"
)

/* ExtensionBlacklist is a list of file extensions that
   are forbidden (blacklisted) in email message payloads */
var ExtensionBlacklist = []string{
	".asd", ".bat", ".chm", ".cmd", ".com", ".dll", ".do", ".exe", ".hlp",
	".hta", ".js", ".jse", ".lnk", ".ocx", ".pif", ".reg", ".scr", ".shb",
	".shm", ".shs", ".vbe", ".vbs", ".vbx", ".vxd", ".wsf", ".wsh", ".xl",
}

// EPayloadNotAllowed is an error that disallows message to pass
var EPayloadNotAllowed = errors.New("552 Message blocked due to blacklisted attachment")

/* AllowFilename returns true if the filename specified does not
   have one of the blacklisted extensions, false otherwise */
func AllowFilename(FileExt string) bool {
	for _, ext := range ExtensionBlacklist {
		if ext == FileExt {
			return false
		}
	}
	return true
}

// ParseMessage processes an email message parts
func ParseEmailMessage(r io.Reader) error {
	// get message from input stream
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return err
	}
	// get media type from email message
	media, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	// accept messages without attachments
	if !strings.HasPrefix(media, "multipart/") {
		return nil
	}
	// deep inspect multipart messages
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		// examine next message part
		part, err := mr.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		// check if part contains a submessage
		if strings.HasPrefix(part.Header.Get("Content-Type"), "message/") {
			// recursively process submessage
			if err := ParseEmailMessage(part); err != nil {
				return err
			}
		}
		// do not process non-attachment parts
		if len(part.FileName()) == 0 {
			continue
		}
		// decode filename
		FileName, err := StringDecode(part.FileName())
		if err != nil {
			return err
		}
		// get file extension
		FileExt := filepath.Ext(strings.ToLower(FileName))
		// check if attachment is blacklisted
		if !AllowFilename(FileExt) {
			// return custom response message
			return EPayloadNotAllowed
		}
		// check archived payload contents
		if SupportedArchive(FileName) {
			// read zip file contents
			slurp, err := ioutil.ReadAll(part)
			if err != nil {
				return err
			}
			// decode base64 contents
			decoded, err := base64.StdEncoding.DecodeString(string(slurp))
			if err != nil {
				return err
			}
			reader := strings.NewReader(string(decoded))
			// examine payload for blacklisted contents
			if err := AllowPayload(FileExt, reader); err != nil {
				return err
			}
		}
	}
	// accept message by default
	return nil
}
