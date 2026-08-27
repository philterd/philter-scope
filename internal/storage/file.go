// Copyright 2026 Philterd, LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/philterd/philterscope/pkg/model"
)

// DefaultHistoryDir is where audit history is kept when MongoDB is not
// configured. It is relative to the working directory.
const DefaultHistoryDir = ".philterscope"

const (
	filePrefix = "audit_"
	fileSuffix = ".json"
)

// An audit ID reaches this package from the ?id= query parameter and is used to
// build a path, so it is restricted to characters that cannot escape the
// history directory. Anything else is rejected rather than sanitized.
var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// FileStorage keeps audit history as one JSON file per audit in a local
// directory. It is the single-user counterpart to MongoDBStorage, which is the
// option for history shared across machines. Concurrent writers are not
// coordinated, so a shared directory needs MongoDB instead.
type FileStorage struct {
	dir string
}

// NewFileStorage returns storage rooted at dir. An empty dir uses
// DefaultHistoryDir. The directory is not created until something is written,
// so serving history from a directory that has none leaves nothing behind.
func NewFileStorage(dir string) *FileStorage {
	if dir == "" {
		dir = DefaultHistoryDir
	}
	return &FileStorage{dir: dir}
}

// Dir returns the directory holding the history.
func (s *FileStorage) Dir() string {
	return s.dir
}

func (s *FileStorage) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid audit ID: %q", id)
	}
	return filepath.Join(s.dir, filePrefix+id+fileSuffix), nil
}

// newID is time-ordered so the files sort usefully in a directory listing, with
// random bytes appended because two audits can finish within the same second.
func newID(t time.Time) string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return t.Format("20060102_150405")
	}
	return t.Format("20060102_150405") + "_" + hex.EncodeToString(b[:])
}

// idFromFilename maps a history file back to its ID. Files written before this
// storage existed are named for their timestamp alone and still resolve.
func idFromFilename(name string) (string, bool) {
	if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix)
	if !validID.MatchString(id) {
		return "", false
	}
	return id, true
}

// historyProjection is the subset of an audit that a history listing needs.
// Decoding into this instead of a full model.AuditResult skips building the
// Details, Overlaps and Spans graph, which dominates an audit file's size and
// is never shown in the listing. It mirrors the projection the MongoDB backend
// asks the server for.
type historyProjection struct {
	Timestamp time.Time              `json:"timestamp"`
	Precision float64                `json:"precision"`
	Recall    float64                `json:"recall"`
	F1Score   float64                `json:"f1_score"`
	Threshold float64                `json:"threshold"`
	Policy    map[string]interface{} `json:"policy"`
	GroupName string                 `json:"group_name"`
}

func (s *FileStorage) readHistoryEntry(id string) (*model.HistoryEntry, error) {
	p, err := s.path(id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit result: %w", err)
	}

	var proj historyProjection
	if err := json.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("failed to parse audit result: %w", err)
	}

	return &model.HistoryEntry{
		ID:        id,
		Timestamp: proj.Timestamp,
		Precision: proj.Precision,
		Recall:    proj.Recall,
		F1Score:   proj.F1Score,
		Threshold: proj.Threshold,
		Policy:    proj.Policy,
		GroupName: proj.GroupName,
	}, nil
}

func (s *FileStorage) read(id string) (*model.AuditResult, error) {
	p, err := s.path(id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit result: %w", err)
	}

	var res model.AuditResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("failed to parse audit result: %w", err)
	}

	// The ID is the filename, so a file copied in by hand or written by an
	// older version still resolves without carrying an id field.
	res.ID = id

	return &res, nil
}

func (s *FileStorage) write(id string, res *model.AuditResult) error {
	p, err := s.path(id)
	if err != nil {
		return err
	}

	res.ID = id
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode audit result: %w", err)
	}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Write to a temporary file first so an interrupted write cannot truncate
	// history that was already saved.
	tmp, err := os.CreateTemp(s.dir, filePrefix+"tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write audit result: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to write audit result: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to save audit result: %w", err)
	}

	return nil
}

// SaveAuditResult writes a new audit to the history directory.
func (s *FileStorage) SaveAuditResult(ctx context.Context, res model.AuditResult) error {
	ts := res.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	return s.write(newID(ts), &res)
}

// GetHistory lists every audit in the directory, most recent first.
func (s *FileStorage) GetHistory(ctx context.Context) ([]model.HistoryEntry, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.HistoryEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	history := []model.HistoryEntry{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := idFromFilename(e.Name())
		if !ok {
			continue
		}
		entry, err := s.readHistoryEntry(id)
		if err != nil {
			// One unreadable file should not hide the rest of the history.
			fmt.Printf("Warning: skipping history file %s: %v\n", e.Name(), err)
			continue
		}
		history = append(history, *entry)
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.After(history[j].Timestamp)
	})

	return history, nil
}

// GetAuditResult returns a single audit by ID.
func (s *FileStorage) GetAuditResult(ctx context.Context, id string) (*model.AuditResult, error) {
	return s.read(id)
}

// DeleteAuditResult removes an audit from the history directory. Deleting an
// audit that is not there succeeds, matching what the MongoDB backend does for
// a delete that matches nothing. A malformed ID is still an error.
func (s *FileStorage) DeleteAuditResult(ctx context.Context, id string) error {
	p, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete audit result: %w", err)
	}
	return nil
}

// setRecommendation applies fn to the recommendation matching entity.
func (s *FileStorage) setRecommendation(id string, entity string, fn func(*model.Recommendation)) error {
	res, err := s.read(id)
	if err != nil {
		return err
	}
	for i := range res.Recommendations {
		if res.Recommendations[i].Entity == entity {
			fn(&res.Recommendations[i])
		}
	}
	return s.write(id, res)
}

// ResolveRecommendation marks a recommendation as resolved.
func (s *FileStorage) ResolveRecommendation(ctx context.Context, auditID string, entity string) error {
	return s.setRecommendation(auditID, entity, func(r *model.Recommendation) { r.Resolved = true })
}

// DismissRecommendation marks a recommendation as dismissed.
func (s *FileStorage) DismissRecommendation(ctx context.Context, auditID string, entity string) error {
	return s.setRecommendation(auditID, entity, func(r *model.Recommendation) { r.Dismissed = true })
}

// SaveAuditNotes stores user-provided notes against an audit.
func (s *FileStorage) SaveAuditNotes(ctx context.Context, id string, notes string) error {
	res, err := s.read(id)
	if err != nil {
		return err
	}
	res.Notes = notes
	return s.write(id, res)
}

// SaveRecommendations replaces the recommendations for an audit.
func (s *FileStorage) SaveRecommendations(ctx context.Context, id string, recs []model.Recommendation) error {
	res, err := s.read(id)
	if err != nil {
		return err
	}
	res.Recommendations = recs
	return s.write(id, res)
}

// Close exists so callers can treat file and MongoDB storage the same way.
func (s *FileStorage) Close(ctx context.Context) error {
	return nil
}
