package main

import (
	"archive/zip"
	"path/filepath"
	"strings"
)

/* AllowZipPayload inspecs a zip attachment in email message and
   returns true if no filenames have a blacklisted extension */
func AllowZipPayload(r *strings.Reader) (bool, error) {
	reader, err := zip.NewReader(r, int64(r.Len()))
	if err != nil {
		return false, err
	}

	// range over filenames in zip archive
	for _, f := range reader.File {
		FileExt := filepath.Ext(strings.ToLower(f.Name))
		if !AllowFilename(FileExt) {
			return false, nil
		}
	}

	return true, nil
}
