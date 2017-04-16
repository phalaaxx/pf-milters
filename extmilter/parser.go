package main

import (
	"encoding/base64"
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
func ParseEmailMessage(r io.Reader) (bool, error) {
	// get message from input stream
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return false, err
	}
	// get media type from email message
	media, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		return false, err
	}
	// accept messages without attachments
	if !strings.HasPrefix(media, "multipart/") {
		return true, nil
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
			return false, err
		}
		// check if part contains a submessage
		if strings.HasPrefix(part.Header.Get("Content-Type"), "message/") {
			// recursively process submessage
			if accept, err := ParseEmailMessage(part); err != nil {
				return false, err
			} else if !accept {
				return false, nil
			}
		}
		// do not process non-attachment parts
		if len(part.FileName()) == 0 {
			continue
		}
		// decode filename
		FileName, err := StringDecode(part.FileName())
		if err != nil {
			return false, err
		}
		// get file extension
		FileExt := filepath.Ext(strings.ToLower(FileName))
		// check if attachment is blacklisted
		if !AllowFilename(FileExt) {
			// return custom response message
			return false, nil
		}
		// define a playload function pointer
		var AllowPayloadFunc func(*strings.Reader) (bool, error)
		// deep inspect archive files
		switch FileExt {
		case ".zip":
			AllowPayloadFunc = AllowZipPayload
		case ".rar":
			AllowPayloadFunc = AllowRarPayload
		//case ".tar":
		//case ".tar.gz":
		//case ".tar.bz2":
		}
		if AllowPayloadFunc != nil {
			// read zip file contents
			slurp, err := ioutil.ReadAll(part)
			if err != nil {
				return false, err
			}
			// decode base64 contents
			decoded, err := base64.StdEncoding.DecodeString(string(slurp))
			if err != nil {
				return false, err
			}
			reader := strings.NewReader(string(decoded))
			// examine payload for blacklisted contents
			if allow, err := AllowPayloadFunc(reader); err != nil {
				return false, err
			} else if !allow {
				// do not allow this message through
				return false, nil
			}
		}
	}
	// accept message by default
	return true, nil
}
