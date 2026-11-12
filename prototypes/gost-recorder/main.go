// Disposable file Recorder experiment. All writes go to a fresh temporary directory.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	core "github.com/go-gost/core/observer/stats"
	"github.com/go-gost/x/config"
	parse "github.com/go-gost/x/config/parsing/recorder"
	stats "github.com/go-gost/x/observer/stats"
	rec "github.com/go-gost/x/recorder"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func read(path string) []byte {
	data, err := os.ReadFile(path)
	check(err)
	return data
}

func rotation(dir string) {
	ctx := context.Background()
	path := filepath.Join(dir, "boundary.jsonl")
	writer := parse.ParseRecorder(&config.RecorderConfig{File: &config.FileRecorder{
		Path: path, Sep: "\n", Rotation: &config.LogRotationConfig{MaxSize: 1},
	}})
	// Warm up the writer to exercise the already-open > comparison.
	check(writer.Record(ctx, []byte("{}")))
	payload := append([]byte(`{"pad":"`), bytes.Repeat([]byte("x"), (1<<20)-3-len(`{"pad":""}`))...)
	payload = append(payload, []byte(`"}`)...)
	check(writer.Record(ctx, payload))
	paths, err := filepath.Glob(filepath.Join(dir, "boundary-*.jsonl"))
	check(err)
	require(len(paths) == 1, "expected one rotated file")
	data := read(paths[0])
	require(len(data) == 1<<20 && bytes.Equal(data[3:], payload), "expected intact JSON at old EOF")
	require(bytes.Equal(read(path), []byte("\n")), "expected separator alone in new file")
	fmt.Printf("rotation=%s old_size=%d old_ends_lf=%v new=%q\n",
		filepath.Base(paths[0]), len(data), data[len(data)-1] == '\n', read(path))
	check(writer.Record(ctx, []byte(`{"next":true}`)))
	check(writer.(interface{ Close() error }).Close())
}

var writeFailure = errors.New("injected write failure")

type faultFile struct {
	*os.File
	remaining int
}

func (f *faultFile) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

func (f *faultFile) Write(p []byte) (int, error) {
	if f.remaining < 0 {
		return f.File.Write(p)
	}
	limit := min(f.remaining, len(p))
	n, err := f.File.Write(p[:limit])
	f.remaining -= n
	if err == nil && n < len(p) {
		err = writeFailure
	}
	return n, err
}

func fault(dir, mode string, budget int) {
	ctx := context.Background()
	path := filepath.Join(dir, mode+".jsonl")
	file, err := os.Create(path)
	check(err)
	out := &faultFile{File: file, remaining: -1}
	writer := rec.FileRecorder(out, rec.SepFileRecorderOption("\n"))
	defer func() { check(writer.(interface{ Close() error }).Close()) }()
	r := rec.NewSessionRecorder(writer, rec.SessionRecorderOptions{Period: time.Hour})
	defer func() { check(r.Close()) }()
	counters := stats.NewStats(false)
	session := r.NewSession(ctx, counters)
	at := time.Date(2026, 11, 12, 15, 4, 5, 123456789, time.FixedZone("UTC+8", 8*3600))
	base := rec.HandlerRecorderObject{Time: at, Network: "tcp", ClientID: "lab"}
	session.Start(base)
	add := func(in, out int64) {
		counters.Add(core.KindInputBytes, in)
		counters.Add(core.KindOutputBytes, out)
	}
	add(100, 200)
	check(session.Interim(ctx, at.Add(time.Second)))
	firstSize := len(read(path))
	add(30, 14)
	out.remaining = budget
	if mode == "separator" {
		// Record 2 has the same encoded size as record 1 except two delta fields.
		out.remaining = firstSize - 1 - 2
	}
	require(errors.Is(session.Interim(ctx, at.Add(2*time.Second)), writeFailure), "expected failure")
	tail := read(path)[firstSize:]
	if mode == "separator" {
		require(json.Valid(tail), "expected complete JSON without LF")
	}
	out.remaining = -1
	add(50, 80)
	check(session.Interim(ctx, at.Add(3*time.Second)))
	// With bytes from the failed write present, the recovery record shares its line.
	data := read(path)
	line := bytes.Split(data, []byte("\n"))[1]
	require(json.Valid(line) == (budget == 0 && mode != "separator"), "unexpected framing")
	var recovery rec.SessionRecord
	check(json.Unmarshal(line[len(tail):], &recovery))
	require(recovery.RecordIndex == 2 && recovery.InputBytesDelta == 80 && recovery.OutputBytesDelta == 94, "unexpected recovery")
	add(1, 2)
	check(session.Interim(ctx, at.Add(4*time.Second)))
	data = read(path)
	require(json.Valid(bytes.Split(data, []byte("\n"))[2]), "expected next physical line to parse")
	// A failed final is terminal even after the file becomes writable.
	base.InputBytes, base.OutputBytes = 200, 300
	out.remaining = 0
	require(errors.Is(session.Finish(ctx, base), writeFailure), "expected failed final")
	out.remaining = -1
	check(session.Finish(ctx, base))
	require(bytes.Equal(data, read(path)), "unexpected final retry")
	fmt.Printf("%s failed_bytes=%d valid_recovery_line=%v failed_final_retried=false\n%s",
		mode, len(tail), json.Valid(line), data)
}

func main() {
	dir, err := os.MkdirTemp("", "gost-recorder-prototype-")
	check(err)
	fmt.Println(dir)
	rotation(dir)
	fault(dir, "no-bytes", 0)
	fault(dir, "partial", 13)
	fault(dir, "separator", -1)
}
