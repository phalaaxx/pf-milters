package main

import (
	"archive/tar"
	"archive/zip"
	"github.com/nwaples/rardecode"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
)

/* SupportedArchive returns true if file name
   represents a supported archive file type */
func SupportedArchive(name string) bool {
	ExtList := []string{"tar", "zip", "rar"}
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
	case ".tar":
		return AllowTarPayload(r)
	case ".zip":
		return AllowZipPayload(r)
	case ".rar":
		return AllowRarPayload(r)
	}
	return nil
}

/* AllowTarPayload inspects a tar attachment in email message and
   returns true if no filenames have a blacklisted extension */
func AllowTarPayload(r *strings.Reader) error {
	// range over tar files
	reader := tar.NewReader(r)
	for {
		// get next file in archive
		header, err := reader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		// check for blacklisted file name
		FileExt := filepath.Ext(strings.ToLower(header.Name))
		if !AllowFilename(FileExt) {
			return EPayloadNotAllowed
		}
		// check for nested archives
		if SupportedArchive(header.Name) {
			slurp, err := ioutil.ReadAll(reader)
			if err != nil {
				// silently ignore errors
				continue
			}
			// check if sub-payload contains any blacklisted files
			if err := AllowPayload(FileExt, strings.NewReader(string(slurp))); err != nil {
				// error, return immediately
				return err
			}
		}
	}
	return nil
}

/* AllowZipPayload inspects a zip attachment in email message and
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
		// check archive within another achive
		if SupportedArchive(f.Name) {
			payload, err := f.Open()
			if err != nil {
				// silently ignore errors
				continue
			}
			// read sub-payload
			slurp, err := ioutil.ReadAll(payload)
			// check if sub-payload contains any blacklisted files
			if err := AllowPayload(FileExt, strings.NewReader(string(slurp))); err != nil {
				// error, return immediately
				return err
			}
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
		// check archive within another achive
		if SupportedArchive(header.Name) {
			slurp, err := ioutil.ReadAll(rr)
			if err != nil {
				// silently ignore errors
				continue
			}
			// check if sub-payload contains any blacklisted files
			if err := AllowPayload(FileExt, strings.NewReader(string(slurp))); err != nil {
				// error, return immediately
				return err
			}
		}
	}
	// no blacklisted file, allow
	return nil
}
