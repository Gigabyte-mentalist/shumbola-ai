// Package audit implements an append-only, tamper-evident audit log
// required by TZ 6.1: every action, tool call, decision, and HITL approval
// must be recorded with a timestamp, and the log must not be alterable
// after the fact.
package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// GenesisHash is the PrevHash value used by the first entry in a log
// (64 hex zero characters — the width of a SHA-256 digest).
var GenesisHash = strings.Repeat("0", 64)

// Entry is a single audit record. Actor identifies who performed the
// action (operator, administrator, or "system"); Action is a short
// verb-based identifier (e.g. "scope.set", "tool.invoke", "hitl.approve");
// Resource identifies what the action targeted (e.g. an engagement or
// target ID); Details carries action-specific structured data.
type Entry struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Resource  string         `json:"resource,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	PrevHash  string         `json:"prev_hash"`
	Hash      string         `json:"hash"`
}

// canonicalPayload returns the deterministic byte representation of the
// entry that gets hashed, excluding the Hash field itself.
func (e Entry) canonicalPayload() ([]byte, error) {
	type payload struct {
		ID        string         `json:"id"`
		Timestamp time.Time      `json:"timestamp"`
		Actor     string         `json:"actor"`
		Action    string         `json:"action"`
		Resource  string         `json:"resource,omitempty"`
		Details   map[string]any `json:"details,omitempty"`
		PrevHash  string         `json:"prev_hash"`
	}
	return json.Marshal(payload{
		ID:        e.ID,
		Timestamp: e.Timestamp,
		Actor:     e.Actor,
		Action:    e.Action,
		Resource:  e.Resource,
		Details:   e.Details,
		PrevHash:  e.PrevHash,
	})
}

func computeHash(e Entry) (string, error) {
	payload, err := e.canonicalPayload()
	if err != nil {
		return "", fmt.Errorf("audit: canonicalize entry: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// Logger appends entries to a JSON-lines file, chaining each entry's hash
// to the previous one so any modification, reordering, or deletion of a
// past line is detectable via Verify.
type Logger struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	lastHash string
	seq      uint64
}

// NewLogger opens (or creates) the audit log at path in append mode and
// primes the hash chain from the last entry already on disk, if any.
func NewLogger(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: open log: %w", err)
	}

	l := &Logger{path: path, file: f, lastHash: GenesisHash}

	entries, err := readEntries(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if len(entries) > 0 {
		l.lastHash = entries[len(entries)-1].Hash
		l.seq = uint64(len(entries))
	}
	// Reset offset to end for subsequent appends (readEntries seeks through the file).
	if _, err := f.Seek(0, os.SEEK_END); err != nil {
		f.Close()
		return nil, fmt.Errorf("audit: seek log: %w", err)
	}
	return l, nil
}

// Append writes a new entry to the log, filling in ID, Timestamp, PrevHash,
// and Hash. It is safe for concurrent use.
func (l *Logger) Append(actor, action, resource string, details map[string]any) (Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	e := Entry{
		ID:        fmt.Sprintf("%d", l.seq),
		Timestamp: time.Now().UTC(),
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Details:   details,
		PrevHash:  l.lastHash,
	}

	hash, err := computeHash(e)
	if err != nil {
		l.seq--
		return Entry{}, err
	}
	e.Hash = hash

	line, err := json.Marshal(e)
	if err != nil {
		l.seq--
		return Entry{}, fmt.Errorf("audit: marshal entry: %w", err)
	}
	line = append(line, '\n')

	if _, err := l.file.Write(line); err != nil {
		l.seq--
		return Entry{}, fmt.Errorf("audit: write entry: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return Entry{}, fmt.Errorf("audit: sync log: %w", err)
	}

	l.lastHash = e.Hash
	return e, nil
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// Verify re-reads the log from disk and checks that every entry's Hash
// matches its recomputed hash and that PrevHash correctly chains to the
// previous entry. It returns an error identifying the first broken link,
// or nil if the whole log is intact.
func Verify(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("audit: open log: %w", err)
	}
	defer f.Close()

	entries, err := readEntries(f)
	if err != nil {
		return err
	}

	prevHash := GenesisHash
	for i, e := range entries {
		if e.PrevHash != prevHash {
			return fmt.Errorf("audit: entry %d (id=%s) has broken chain: expected prev_hash %s, got %s", i, e.ID, prevHash, e.PrevHash)
		}
		wantHash, err := computeHash(e)
		if err != nil {
			return err
		}
		if e.Hash != wantHash {
			return fmt.Errorf("audit: entry %d (id=%s) has invalid hash: tampered content", i, e.ID)
		}
		prevHash = e.Hash
	}
	return nil
}

func readEntries(f *os.File) ([]Entry, error) {
	if _, err := f.Seek(0, os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("audit: seek log: %w", err)
	}
	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("audit: parse log line: %w", err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("audit: read log: %w", err)
	}
	return entries, nil
}
