package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"
)

func fixedClock(milliseconds int64) clock {
	return func() (int64, error) { return milliseconds, nil }
}

func uuidBytes(t *testing.T, uuid string) []byte {
	t.Helper()
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(uuid) {
		t.Fatalf("invalid canonical UUIDv7: %q", uuid)
	}
	data, err := hex.DecodeString(strings.ReplaceAll(uuid, "-", ""))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func timestamp(data []byte) int64 {
	var value int64
	for _, b := range data[:6] {
		value = value<<8 | int64(b)
	}
	return value
}

func TestKnownVectors(t *testing.T) {
	cases := []struct {
		ms      int64
		entropy []byte
		uuid    string
	}{
		{0, make([]byte, 10), "00000000-0000-7000-8000-000000000000"},
		{maxTimestamp, bytes.Repeat([]byte{0xff}, 10), "ffffffff-ffff-7fff-bfff-ffffffffffff"},
		{0x017f22e279b0, []byte{0xcc, 0xc3, 0xd8, 0xc4, 0xdc, 0x0c, 0x0c, 0x07, 0x39, 0x8f},
			"017f22e2-79b0-7cc3-98c4-dc0c0c07398f"},
		{0x010203040506, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a},
			"01020304-0506-7102-8304-05060708090a"},
	}
	for _, tc := range cases {
		uuid, err := generateUUID(fixedClock(tc.ms), bytes.NewReader(tc.entropy))
		if err != nil || uuid != tc.uuid {
			t.Fatalf("generateUUID(%d) = %q, %v; want %q", tc.ms, uuid, err, tc.uuid)
		}
		if got := timestamp(uuidBytes(t, uuid)); got != tc.ms {
			t.Fatalf("timestamp = %d, want %d", got, tc.ms)
		}
	}
}

func TestTimestampBoundaries(t *testing.T) {
	values := []int64{0, 1, maxTimestamp - 1, maxTimestamp}
	for bits := uint(8); bits < 48; bits += 8 {
		values = append(values, 1<<bits-1, 1<<bits, 1<<bits+1)
	}
	for _, ms := range values {
		uuid, err := generateUUID(fixedClock(ms), bytes.NewReader(make([]byte, 10)))
		if err != nil {
			t.Fatal(err)
		}
		if got := timestamp(uuidBytes(t, uuid)); got != ms {
			t.Fatalf("timestamp = %d, want %d", got, ms)
		}
	}
}

func TestRandomFieldMasks(t *testing.T) {
	for value := 0; value < 256; value++ {
		entropy := bytes.Repeat([]byte{byte(value)}, 10)
		reader := bytes.NewReader(append(entropy, 0xaa))
		uuid, err := generateUUID(fixedClock(42), reader)
		if err != nil {
			t.Fatal(err)
		}
		data := uuidBytes(t, uuid)
		entropy[0] = entropy[0]&0x0f | 0x70
		entropy[2] = entropy[2]&0x3f | 0x80
		if !bytes.Equal(data[6:], entropy) || reader.Len() != 1 {
			t.Fatalf("random bits were lost or wrong entropy length: %x", data)
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy device unavailable")
}

type chunkReader struct{ remaining int }

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	p[0] = 0x55
	r.remaining--
	return 1, nil
}

func TestEntropyReads(t *testing.T) {
	for length := 0; length < 10; length++ {
		uuid, err := generateUUID(fixedClock(0), bytes.NewReader(make([]byte, length)))
		diagnostic := "unexpected EOF"
		if length == 0 {
			diagnostic = "EOF"
		}
		if uuid != "" || err == nil || err.Code != "entropy_error" || !strings.Contains(err.Message, diagnostic) {
			t.Fatalf("short read %d = %q, %v", length, uuid, err)
		}
	}
	uuid, err := generateUUID(fixedClock(0), &chunkReader{10})
	if err != nil || uuid != "00000000-0000-7555-9555-555555555555" {
		t.Fatalf("chunked read = %q, %v", uuid, err)
	}
}

func TestGenerationFailureProtocol(t *testing.T) {
	cases := []struct {
		now     clock
		entropy io.Reader
		code    string
		message string
	}{
		{fixedClock(-1), failingReader{}, "clock_error", "Unix millisecond timestamp -1 is outside [0, 281474976710655]"},
		{fixedClock(maxTimestamp + 1), failingReader{}, "clock_error", "Unix millisecond timestamp 281474976710656 is outside [0, 281474976710655]"},
		{func() (int64, error) { return 0, errors.New("clock \"unavailable\"\n") },
			failingReader{}, "clock_error", "read Unix millisecond clock: clock \"unavailable\"\n"},
		{fixedClock(0), failingReader{}, "entropy_error", "read UUID entropy: entropy device unavailable"},
		{fixedClock(0), bytes.NewReader([]byte{1}), "entropy_error", "read UUID entropy: unexpected EOF"},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		status := run([]string{"generate"}, &stdout, &stderr, tc.now, tc.entropy)
		want, _ := json.Marshal(struct {
			Protocol int             `json:"protocol"`
			Error    generationError `json:"error"`
		}{1, generationError{tc.code, tc.message}})
		if status != 1 || stdout.Len() != 0 || stderr.String() != string(want)+"\n" {
			t.Fatalf("failure = %d, %q, %q; want %s", status, stdout.String(), stderr.String(), want)
		}
	}
}

func TestCommandProtocol(t *testing.T) {
	cases := []struct {
		args   []string
		status int
		output string
	}{
		{[]string{"generate"}, 0, "{\"protocol\":1,\"uuid\":\"00000000-0000-7000-8000-000000000000\"}\n"},
		{[]string{"--version"}, 0, "{\"protocol\":1,\"version\":\"0.1.0\"}\n"},
		{[]string{"--help"}, 0, "{\"protocol\":1,\"usage\":\"uuid-test generate | --version | --help\"}\n"},
		{nil, 2, ""},
		{[]string{""}, 2, ""},
		{[]string{"greet", "--name", "World"}, 2, ""},
		{[]string{"Generate"}, 2, ""},
		{[]string{"generate", "extra"}, 2, ""},
		{[]string{"--help", "--version"}, 2, ""},
		{[]string{"\xff"}, 2, ""},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		status := run(tc.args, &stdout, &stderr, fixedClock(0), bytes.NewReader(make([]byte, 10)))
		wantError := ""
		if tc.status == 2 {
			wantError = invalidArguments + "\n"
		}
		if status != tc.status || stdout.String() != tc.output || stderr.String() != wantError {
			t.Fatalf("run(%q) = %d, %q, %q", tc.args, status, stdout.String(), stderr.String())
		}
	}
}

type failingWriter struct {
	err   error
	calls int
}

func (w *failingWriter) Write(data []byte) (int, error) {
	w.calls++
	return len(data) / 2, w.err
}

func TestSuccessOutputFailures(t *testing.T) {
	for _, command := range []string{"--help", "--version", "generate"} {
		for _, failure := range []error{errors.New("output \"unavailable\"\n"), nil} {
			t.Run(command+"/"+fmt.Sprint(failure), func(t *testing.T) {
				stdout := &failingWriter{err: failure}
				var stderr bytes.Buffer
				status := run([]string{command}, stdout, &stderr, fixedClock(0), bytes.NewReader(make([]byte, 10)))
				diagnostic := failure
				if diagnostic == nil {
					diagnostic = io.ErrShortWrite
				}
				want, err := json.Marshal(struct {
					Protocol int             `json:"protocol"`
					Error    generationError `json:"error"`
				}{1, generationError{"output_error", "write stdout: " + diagnostic.Error()}})
				if err != nil {
					t.Fatal(err)
				}
				if status != 1 || stdout.calls != 1 || stderr.String() != string(want)+"\n" {
					t.Fatalf("output failure = %d, %d writes, %q; want %s", status, stdout.calls, stderr.String(), want)
				}
			})
		}
	}
}

func TestErrorOutputFailures(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		now     clock
		entropy func() io.Reader
		output  io.Writer
	}{
		{"missing arguments", nil, fixedClock(0), func() io.Reader { return nil }, io.Discard},
		{"unknown command", []string{"unknown"}, fixedClock(0), func() io.Reader { return nil }, io.Discard},
		{"clock error", []string{"generate"}, fixedClock(-1), func() io.Reader { return nil }, io.Discard},
		{"entropy error", []string{"generate"}, fixedClock(0), func() io.Reader { return failingReader{} }, io.Discard},
		{"stdout and stderr errors", []string{"generate"}, fixedClock(0),
			func() io.Reader { return bytes.NewReader(make([]byte, 10)) },
			&failingWriter{err: errors.New("stdout unavailable")}},
	}
	for _, tc := range cases {
		for _, failure := range []error{errors.New("stderr unavailable"), nil} {
			t.Run(tc.name+"/"+fmt.Sprint(failure), func(t *testing.T) {
				stderr := &failingWriter{err: failure}
				status := run(tc.args, tc.output, stderr, tc.now, tc.entropy())
				if status != 1 || stderr.calls != 1 {
					t.Fatalf("error output failure = %d, %d stderr writes", status, stderr.calls)
				}
			})
		}
	}
}

func TestUnusedStderrFailure(t *testing.T) {
	for _, command := range []string{"--help", "--version", "generate"} {
		var stdout bytes.Buffer
		stderr := &failingWriter{err: errors.New("stderr unavailable")}
		status := run([]string{command}, &stdout, stderr, fixedClock(0), bytes.NewReader(make([]byte, 10)))
		if status != 0 || stdout.Len() == 0 || stderr.calls != 0 {
			t.Fatalf("%s used stderr: %d, %d writes", command, status, stderr.calls)
		}
	}
}

func TestSameMillisecondCollisions(t *testing.T) {
	const count = 10000
	const ms = int64(1700000000123)
	seen := make(map[string]bool, count)
	for i := 0; i < count; i++ {
		uuid, err := generateUUID(fixedClock(ms), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if seen[uuid] {
			t.Fatalf("collision at sample %d: %s", i, uuid)
		}
		seen[uuid] = true
		if timestamp(uuidBytes(t, uuid)) != ms {
			t.Fatal("incorrect fixed timestamp")
		}
	}
}

func TestRealClockBounds(t *testing.T) {
	before := time.Now().UnixMilli()
	uuid, err := generateUUID(func() (int64, error) { return time.Now().UnixMilli(), nil }, rand.Reader)
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatal(err)
	}
	ms := timestamp(uuidBytes(t, uuid))
	if ms < before || ms > after {
		t.Fatalf("timestamp %d outside [%d, %d]", ms, before, after)
	}
}
