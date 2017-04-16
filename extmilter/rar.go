package main

import (
	"github.com/nwaples/rardecode"
	"io"
	"path/filepath"
	"strings"
)

/* AllowRarPayload inspects a rar attachment in email message and
   returns true if no filenames have a blacklisted extension */
func AllowRarPayload(r *strings.Reader) (bool, error) {
	// make rar file reader object
	rr, err := rardecode.NewReader(r, "")
	if err != nil {
		return false, err
	}
	// walk files in archive
	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return false, err
		}
		// compare current name against blacklisted extensions
		FileExt := filepath.Ext(strings.ToLower(header.Name))
		if !AllowFilename(FileExt) {
			return false, nil
		}
	}
	// no blacklisted file, allow
	return true, nil
}
