package main

import (
	"archive/zip"
	"github.com/nwaples/rardecode"
	"io"
	"path/filepath"
	"strings"
)

/* SupportedArchive returns true if file name
   represents a supported archive file type */
func SupportedArchive(name string) bool {
	ExtList := []string{"zip", "rar"}
	nameLower := strings.ToLower(name)
	for _, ext := range ExtList {
		if strings.HasSuffix(nameLower, ext) {
			return true
		}
	}
	return false
}

/* AllowPayload runs the appropriate decompress
   function according to provided extension */
func AllowPayload(ext string, r *strings.Reader) error {
	switch ext {
	case ".zip":
		return AllowZipPayload(r)
	case ".rar":
		return AllowRarPayload(r)
	}
	return nil
}

/* AllowZipPayload inspecs a zip attachment in email message and
   returns true if no filenames have a blacklisted extension */
func AllowZipPayload(r *strings.Reader) error {
	reader, err := zip.NewReader(r, int64(r.Len()))
	if err != nil {
		return err
	}

	// range over filenames in zip archive
	for _, f := range reader.File {
		FileExt := filepath.Ext(strings.ToLower(f.Name))
		if !AllowFilename(FileExt) {
			return EPayloadNotAllowed
		}
	}

	return nil
}

/* AllowRarPayload inspects a rar attachment in email message and
   returns true if no filenames have a blacklisted extension */
func AllowRarPayload(r *strings.Reader) error {
	// make rar file reader object
	rr, err := rardecode.NewReader(r, "")
	if err != nil {
		return err
	}
	// walk files in archive
	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		// compare current name against blacklisted extensions
		FileExt := filepath.Ext(strings.ToLower(header.Name))
		if !AllowFilename(FileExt) {
			return EPayloadNotAllowed
		}
	}
	// no blacklisted file, allow
	return nil
}
