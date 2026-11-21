package collect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LogSource struct {
	Files        []string
	LivePath     string
	Interval     time.Duration
	PollInterval time.Duration
	Report       Reporter
}

func (*LogSource) Name() string { return "log-replay" }
func (s *LogSource) Stream(ctx context.Context) <-chan RawSample {
	out := make(chan RawSample)
	go func() {
		defer close(out)
		if err := validateInterval(s.Interval); err != nil {
			report(s.Report, err)
			return
		}
		poll := s.PollInterval
		if poll <= 0 {
			poll = 100 * time.Millisecond
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); s.appendReplay(ctx) }()
		defer wg.Wait()
		tail := &LogTail{}
		for ctx.Err() == nil {
			lines, err := tail.Read(s.LivePath)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				report(s.Report, err)
			}
			for _, line := range lines {
				source := Replayed
				if strings.HasPrefix(line, "KALKI_DEMO ") {
					source = Simulated
				}
				if !emit(ctx, out, RawSample{CollectedAt: Timestamp(time.Now()), Source: source, Log: line, StreamID: s.LivePath}) {
					return
				}
			}
			if !pause(ctx, poll) {
				return
			}
		}
	}()
	return out
}

func (s *LogSource) appendReplay(ctx context.Context) {
	if err := os.MkdirAll(filepath.Dir(s.LivePath), 0700); err != nil {
		report(s.Report, err)
		return
	}
	for ctx.Err() == nil {
		for _, path := range s.Files {
			if ctx.Err() != nil {
				return
			}
			f, err := os.Open(path)
			if err != nil {
				report(s.Report, err)
				continue
			}
			err = s.appendFile(ctx, f)
			report(s.Report, f.Close())
			report(s.Report, err)
		}
		if !pause(ctx, s.Interval) {
			return
		}
	}
}

func (s *LogSource) appendFile(ctx context.Context, input io.Reader) error {
	scan := bufio.NewScanner(input)
	scan.Buffer(make([]byte, 4096), 1024*1024)
	for scan.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		line := scan.Text()
		// Retry the same line on a local write failure; never skip it silently.
		for ctx.Err() == nil {
			f, err := os.OpenFile(s.LivePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err == nil {
				_, err = io.WriteString(f, line+"\n")
				closeErr := f.Close()
				if err == nil {
					err = closeErr
				}
			}
			if err == nil {
				break
			}
			report(s.Report, err)
			if !pause(ctx, s.Interval) {
				return nil
			}
		}
		if !pause(ctx, s.Interval) {
			return nil
		}
	}
	return scan.Err()
}

// LogTail preserves partial lines between polls and resets on replacement,
// truncation, or a changed anchor before the previous read position.
type LogTail struct {
	offset  int64
	pending string
	info    os.FileInfo
	anchor  string
}

func (t *LogTail) Read(path string) (result []string, resultErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	reset := t.info != nil && (!os.SameFile(t.info, info) || info.Size() < t.offset)
	if !reset && t.anchor != "" {
		buf := make([]byte, len(t.anchor))
		_, err := f.ReadAt(buf, t.offset-int64(len(buf)))
		reset = err != nil || string(buf) != t.anchor
	}
	if reset {
		t.offset = 0
		t.pending = ""
		t.anchor = ""
	}
	t.info = info
	if len(t.pending) > 1024*1024 {
		return nil, fmt.Errorf("local log line exceeds 1 MiB; tail paused until file rotation/truncation")
	}
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil, err
	}
	// Bound each poll without discarding unread bytes.
	data, err := io.ReadAll(io.LimitReader(f, 64*1024))
	if err != nil {
		return nil, err
	}
	t.offset += int64(len(data))
	anchor := t.anchor + string(data)
	if len(anchor) > 64 {
		anchor = anchor[len(anchor)-64:]
	}
	t.anchor = anchor
	text := t.pending + string(data)
	parts := strings.Split(text, "\n")
	t.pending = parts[len(parts)-1]
	lines := parts[:len(parts)-1]
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if len(t.pending) > 1024*1024 {
		return lines, fmt.Errorf("local log line exceeds 1 MiB; tail paused until file rotation/truncation")
	}
	return lines, nil
}
