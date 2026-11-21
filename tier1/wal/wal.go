// Package wal implements a checksummed append-only journal. Acknowledged batch
// bytes remain available to rebuild an in-memory receiver after its restart.
package wal

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"kalki/tier1/policy"
)

var ErrFull = errors.New("BUFFER_FULL: WAL capacity reached; no new batch accepted")

type Binding struct {
	Tenant      string `json:"tenant"`
	CollectorID string `json:"collector_id"`
	Region      string `json:"region"`
}
type Entry struct {
	Seq      uint64 `json:"seq"`
	Bytes    []byte `json:"bytes"`
	Hash     string `json:"sha256"`
	Stripped int    `json:"stripped_count"`
}
type frame struct {
	Kind    string   `json:"kind"`
	Binding *Binding `json:"binding,omitempty"`
	Batch   *Entry   `json:"batch,omitempty"`
	Seq     uint64   `json:"seq,omitempty"`
	Epoch   string   `json:"epoch,omitempty"`
}
type Journal struct {
	mu           sync.Mutex
	f            *os.File
	binding      Binding
	entries      []Entry
	acked        map[uint64]bool
	epoch        string
	size, cap    int64
	broken       bool
	RepairedTail bool
}

func Open(path string, binding Binding, capacity int64) (*Journal, error) {
	if capacity < 4096 {
		return nil, fmt.Errorf("WAL capacity must be at least 4096 bytes")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("WAL directory unavailable")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("WAL open failed")
	}
	if err := lock(f); err != nil {
		closeErr := f.Close()
		return nil, errors.Join(err, closeErr)
	}
	j := &Journal{f: f, binding: binding, cap: capacity, acked: map[uint64]bool{}}
	if err := j.recover(); err != nil {
		return nil, errors.Join(err, j.Close())
	}
	if j.size > j.cap {
		return nil, errors.Join(ErrFull, j.Close())
	}
	if j.size == 0 {
		if err := j.append(frame{Kind: "identity", Binding: &binding}, false); err != nil {
			return nil, errors.Join(err, j.Close())
		}
	}
	return j, nil
}

func (j *Journal) recover() error {
	var offset int64
	for {
		header := make([]byte, 36)
		n, err := io.ReadFull(j.f, header)
		if err == io.EOF && n == 0 {
			break
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return j.repair(offset)
		}
		if err != nil {
			return fmt.Errorf("WAL read failed")
		}
		length := binary.BigEndian.Uint32(header[:4])
		if length == 0 || length > 2*1024*1024 {
			return fmt.Errorf("WAL corrupt frame length")
		}
		data := make([]byte, int(length))
		_, err = io.ReadFull(j.f, data)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return j.repair(offset)
		}
		if err != nil {
			return fmt.Errorf("WAL read failed")
		}
		sum := sha256.Sum256(data)
		if string(sum[:]) != string(header[4:]) {
			return fmt.Errorf("WAL checksum mismatch; recovery stopped")
		}
		var event frame
		if json.Unmarshal(data, &event) != nil {
			return fmt.Errorf("WAL invalid frame")
		}
		switch event.Kind {
		case "identity":
			if offset != 0 || event.Binding == nil || *event.Binding != j.binding {
				return fmt.Errorf("WAL identity mismatch")
			}
		case "batch":
			if offset == 0 || event.Batch == nil || event.Batch.Seq != uint64(len(j.entries)+1) {
				return fmt.Errorf("WAL sequence corruption")
			}
			sum := sha256.Sum256(event.Batch.Bytes)
			if hex.EncodeToString(sum[:]) != event.Batch.Hash {
				return fmt.Errorf("WAL payload corruption")
			}
			j.entries = append(j.entries, *event.Batch)
		case "epoch":
			if offset == 0 || event.Epoch == "" {
				return fmt.Errorf("WAL epoch corruption")
			}
			j.epoch = event.Epoch
			j.acked = map[uint64]bool{}
		case "ack":
			if event.Epoch != j.epoch || event.Seq == 0 || event.Seq > uint64(len(j.entries)) {
				return fmt.Errorf("WAL ACK corruption")
			}
			j.acked[event.Seq] = true
		default:
			return fmt.Errorf("WAL unknown frame")
		}
		offset += 36 + int64(length)
	}
	j.size = offset
	if j.size > j.cap {
		return ErrFull
	}
	return nil
}

func (j *Journal) repair(offset int64) error {
	if err := j.f.Truncate(offset); err != nil {
		return fmt.Errorf("WAL incomplete tail repair failed")
	}
	if _, err := j.f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	if err := j.f.Sync(); err != nil {
		return err
	}
	j.size = offset
	j.RepairedTail = true
	return nil
}

func (j *Journal) append(event frame, reserve bool) error {
	if j.broken {
		return fmt.Errorf("WAL unavailable after write failure")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	extra := int64(0)
	if reserve {
		extra = int64(len(j.entries)-len(j.acked)+2)*256 + 2048 // emergency status record
	}
	if j.size+int64(len(data)+36)+extra > j.cap {
		return ErrFull
	}
	buf := make([]byte, 36+len(data))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data)))
	sum := sha256.Sum256(data)
	copy(buf[4:36], sum[:])
	copy(buf[36:], data)
	n, err := j.f.Write(buf)
	if err == nil && n != len(buf) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = j.f.Sync()
	}
	if err != nil {
		j.broken = true
		return fmt.Errorf("WAL write/sync failed; restart required")
	}
	j.size += int64(len(buf))
	return nil
}

// Enqueue assigns seq inside the journal lock, but advances it only after fsync.
func (j *Journal) Enqueue(build func(uint64) (policy.Approved, error), priorStripped ...int) (Entry, error) {
	return j.enqueue(build, true, priorStripped...)
}

// EnqueueEmergency uses reserved space for a policy-approved BUFFER_FULL record.
// It still appends and fsyncs before transmission; disk failure remains fatal.
func (j *Journal) EnqueueEmergency(build func(uint64) (policy.Approved, error)) (Entry, error) {
	return j.enqueue(build, false)
}
func (j *Journal) enqueue(build func(uint64) (policy.Approved, error), reserve bool, priorStripped ...int) (Entry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	seq := uint64(len(j.entries) + 1)
	approved, err := build(seq)
	if err != nil {
		return Entry{}, err
	}
	if len(approved.Bytes()) == 0 {
		return Entry{}, fmt.Errorf("WAL requires policy-approved bytes")
	}
	e := Entry{Seq: seq, Bytes: approved.Bytes(), Hash: approved.SHA256(), Stripped: approved.StrippedCount()}
	if len(priorStripped) > 0 {
		e.Stripped += priorStripped[0]
	}
	if err := j.append(frame{Kind: "batch", Batch: &e}, reserve); err != nil {
		return Entry{}, err
	}
	j.entries = append(j.entries, e)
	return clone(e), nil
}

func (j *Journal) IsAcked(seq uint64) bool { j.mu.Lock(); defer j.mu.Unlock(); return j.acked[seq] }
func clone(e Entry) Entry                  { e.Bytes = append([]byte(nil), e.Bytes...); return e }
func (j *Journal) Entries() []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Entry, len(j.entries))
	for i, e := range j.entries {
		out[i] = clone(e)
	}
	return out
}
func (j *Journal) Next() (Entry, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, e := range j.entries {
		if !j.acked[e.Seq] {
			return clone(e), true
		}
	}
	return Entry{}, false
}
func (j *Journal) Depth() int      { j.mu.Lock(); defer j.mu.Unlock(); return len(j.entries) - len(j.acked) }
func (j *Journal) LastSeq() uint64 { j.mu.Lock(); defer j.mu.Unlock(); return uint64(len(j.entries)) }
func (j *Journal) Epoch() string   { j.mu.Lock(); defer j.mu.Unlock(); return j.epoch }
func (j *Journal) SetEpoch(epoch string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if epoch == j.epoch {
		return nil
	}
	if epoch == "" {
		return fmt.Errorf("empty receiver epoch")
	}
	if err := j.append(frame{Kind: "epoch", Epoch: epoch}, false); err != nil {
		return err
	}
	j.epoch = epoch
	j.acked = map[uint64]bool{}
	return nil
}
func (j *Journal) Ack(seq uint64, epoch, hash string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if epoch != j.epoch || seq == 0 || seq > uint64(len(j.entries)) || j.entries[seq-1].Hash != hash {
		return fmt.Errorf("invalid ACK; WAL retained")
	}
	if j.acked[seq] {
		return nil
	}
	if err := j.append(frame{Kind: "ack", Seq: seq, Epoch: epoch}, false); err != nil {
		return err
	}
	j.acked[seq] = true
	return nil
}
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.f == nil {
		return nil
	}
	err := j.f.Close()
	j.f = nil
	return err
}
