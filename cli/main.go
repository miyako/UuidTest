package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

const invalidArguments = `{"protocol":1,"error":{"code":"invalid_arguments","message":"expected generate, --version or --help"}}`
const maxTimestamp int64 = 1<<48 - 1

type clock func() (int64, error)

type generationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func generateUUID(now clock, entropy io.Reader) (string, *generationError) {
	milliseconds, err := now()
	if err != nil {
		return "", &generationError{"clock_error", fmt.Sprintf("read Unix millisecond clock: %v", err)}
	}
	if milliseconds < 0 || milliseconds > maxTimestamp {
		return "", &generationError{
			"clock_error",
			fmt.Sprintf("Unix millisecond timestamp %d is outside [0, %d]", milliseconds, maxTimestamp),
		}
	}
	var uuid [16]byte
	for i := 0; i < 6; i++ {
		uuid[i] = byte(milliseconds >> (8 * (5 - i)))
	}
	if _, err := io.ReadFull(entropy, uuid[6:]); err != nil {
		return "", &generationError{"entropy_error", fmt.Sprintf("read UUID entropy: %v", err)}
	}
	uuid[6] = uuid[6]&0x0f | 0x70
	uuid[8] = uuid[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

type checkedWriter struct{ io.Writer }

func (w checkedWriter) Write(data []byte) (int, error) {
	n, err := w.Writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}

func writeError(stderr io.Writer, diagnostic *generationError) error {
	return json.NewEncoder(checkedWriter{stderr}).Encode(struct {
		Protocol int              `json:"protocol"`
		Error    *generationError `json:"error"`
	}{1, diagnostic})
}

func rejectArguments(stderr io.Writer) int {
	if _, err := io.WriteString(checkedWriter{stderr}, invalidArguments+"\n"); err != nil {
		return 1
	}
	return 2
}

func run(args []string, stdout, stderr io.Writer, now clock, entropy io.Reader) int {
	if len(args) != 1 {
		return rejectArguments(stderr)
	}
	var output string
	switch args[0] {
	case "--version":
		output = `{"protocol":1,"version":"0.1.0"}`
	case "--help":
		output = `{"protocol":1,"usage":"uuid-test generate | --version | --help"}`
	case "generate":
		uuid, err := generateUUID(now, entropy)
		if err != nil {
			// A failed stderr write cannot be reported on that same stream.
			_ = writeError(stderr, err)
			return 1
		}
		output = fmt.Sprintf("{\"protocol\":1,\"uuid\":\"%s\"}", uuid)
	default:
		return rejectArguments(stderr)
	}
	if _, err := io.WriteString(checkedWriter{stdout}, output+"\n"); err != nil {
		_ = writeError(stderr, &generationError{"output_error", fmt.Sprintf("write stdout: %v", err)})
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, func() (int64, error) {
		return time.Now().UnixMilli(), nil
	}, rand.Reader))
}
