package main

import (
	"bytes"
	"compress/gzip"
	"io"
)

func GetUnzipContent(zipContent []byte) ([]byte, error) {
	buf := bytes.NewBuffer(zipContent)
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	unzippedContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return unzippedContent, nil
}
