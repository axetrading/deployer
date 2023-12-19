package deployer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"
)

// linesResult represents the result of streaming line groups.
type linesResult struct {
	lines []string
	err   error
}

// streamLineGroups reads from the reader and streams line groups through a channel.
func streamLineGroups(reader io.ReadCloser) chan *linesResult {
	result := make(chan *linesResult)
	go func() {
		defer close(result)
		remainder := []byte{}
		for {
			buf := make([]byte, 5*1024)
			n, err := reader.Read(buf)
			if err == io.EOF {
				if n != 0 {
					log.Fatalf("unreachable: unexpected n > 0 on EOF")
				}
				break
			}
			if err != nil {
				result <- &linesResult{err: fmt.Errorf("read error: %w", err)}
				return
			}
			data := append(remainder, buf[0:n]...)
			remainder = []byte{}
			lines := []string{}
			i := 0
			for i < len(data) {
				newlineAt := bytes.IndexByte(data[i:], '\n')
				if newlineAt == -1 {
					remainder = data[i:]
					break
				}
				lines = append(lines, string(data[i:i+newlineAt]))
				i += newlineAt + 1
			}
			result <- &linesResult{lines: lines}
		}
		if len(remainder) != 0 {
			result <- &linesResult{lines: []string{string(remainder)}}
		}
	}()
	return result
}

// unzip extracts the contents of the archive to the destination directory.
func unzip(archivePath, destDir string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, item := range archive.File {
		if item.Mode().IsDir() {
			continue
		}
		reader, err := item.Open()
		if err != nil {
			return err
		}
		name := path.Join(destDir, item.Name)
		if err := os.MkdirAll(path.Dir(name), os.ModeDir); err != nil {
			return err
		}
		writer, err := os.Create(name)
		if err := writer.Chmod(item.Mode()); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		defer writer.Close()
		if _, err := io.Copy(writer, reader); err != nil {
			return err
		}
	}
	return nil
}

// copy copies a file from source to destination.
func copy(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

// makeExecutable makes a file executable.
func makeExecutable(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	return os.Chmod(filename, info.Mode()|0111)
}

// waitForFileToExist waits for a file to exist.
func waitForFileToExist(filename string) error {
	for {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			time.Sleep(1 * time.Second)
		} else if err != nil {
			return err
		} else {
			return nil
		}
	}
}

// createLinesLogData creates log data from the lines result.
func createLinesLogData(result *linesResult, done bool) []byte {
	var errorMessage *string = nil
	if result.err != nil {
		message := result.err.Error()
		errorMessage = &message
	}
	data := logData{
		Lines: result.lines,
		Error: errorMessage,
		Done:  done || result.err != nil,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalln("failed to marshal log data: " + err.Error())
	}
	return jsonData
}

// logData represents the log data structure.
type logData struct {
	Lines []string `json:"lines"`
	Done  bool     `json:"done"`
	Error *string  `json:"error"`
}

// logResposne represents the log response structure.
type logResposne struct {
	ContinueURL string `json:"continue"`
}
