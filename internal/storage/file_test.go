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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/philterd/philterscope/pkg/model"
)

func newTestStorage(t *testing.T) *FileStorage {
	t.Helper()
	return NewFileStorage(t.TempDir())
}

func saveOne(t *testing.T, s *FileStorage, res model.AuditResult) string {
	t.Helper()
	if err := s.SaveAuditResult(context.Background(), res); err != nil {
		t.Fatalf("SaveAuditResult: %v", err)
	}
	history, err := s.GetHistory(context.Background())
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("expected at least one history entry")
	}
	return history[0].ID.(string)
}

func TestFileStorageRoundTrip(t *testing.T) {
	s := newTestStorage(t)

	id := saveOne(t, s, model.AuditResult{
		Timestamp: time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC),
		Precision: 0.82,
		Recall:    0.68,
		F1Score:   0.74,
		GroupName: "nightly",
		Recommendations: []model.Recommendation{
			{Entity: "NAME", Description: "raise sensitivity"},
			{Entity: "SSN", Description: "add a filter"},
		},
	})

	res, err := s.GetAuditResult(context.Background(), id)
	if err != nil {
		t.Fatalf("GetAuditResult: %v", err)
	}
	if res.F1Score != 0.74 || res.GroupName != "nightly" {
		t.Errorf("unexpected result: %+v", res)
	}
	if res.ID != id {
		t.Errorf("expected ID %q, got %v", id, res.ID)
	}
}

func TestFileStorageHistoryIsNewestFirst(t *testing.T) {
	s := newTestStorage(t)
	base := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)

	for i, ts := range []time.Time{base, base.Add(2 * time.Hour), base.Add(time.Hour)} {
		if err := s.SaveAuditResult(context.Background(), model.AuditResult{
			Timestamp: ts,
			F1Score:   float64(i),
		}); err != nil {
			t.Fatalf("SaveAuditResult: %v", err)
		}
	}

	history, err := s.GetHistory(context.Background())
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(history))
	}
	for i := 1; i < len(history); i++ {
		if history[i-1].Timestamp.Before(history[i].Timestamp) {
			t.Errorf("history is not newest first: %v before %v",
				history[i-1].Timestamp, history[i].Timestamp)
		}
	}
}

func TestFileStorageEmptyHistory(t *testing.T) {
	s := newTestStorage(t)
	history, err := s.GetHistory(context.Background())
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if history == nil {
		t.Error("expected an empty slice, not nil, so the UI renders an empty list")
	}
	if len(history) != 0 {
		t.Errorf("expected no entries, got %d", len(history))
	}
}

func TestFileStorageMutations(t *testing.T) {
	ctx := context.Background()
	s := newTestStorage(t)

	id := saveOne(t, s, model.AuditResult{
		Timestamp: time.Now(),
		Recommendations: []model.Recommendation{
			{Entity: "NAME"},
			{Entity: "SSN"},
		},
	})

	if err := s.ResolveRecommendation(ctx, id, "NAME"); err != nil {
		t.Fatalf("ResolveRecommendation: %v", err)
	}
	if err := s.DismissRecommendation(ctx, id, "SSN"); err != nil {
		t.Fatalf("DismissRecommendation: %v", err)
	}
	if err := s.SaveAuditNotes(ctx, id, "checked by hand"); err != nil {
		t.Fatalf("SaveAuditNotes: %v", err)
	}

	res, err := s.GetAuditResult(ctx, id)
	if err != nil {
		t.Fatalf("GetAuditResult: %v", err)
	}
	if !res.Recommendations[0].Resolved {
		t.Error("NAME recommendation should be resolved")
	}
	if res.Recommendations[0].Dismissed {
		t.Error("NAME recommendation should not be dismissed")
	}
	if !res.Recommendations[1].Dismissed {
		t.Error("SSN recommendation should be dismissed")
	}
	if res.Notes != "checked by hand" {
		t.Errorf("expected the notes to persist, got %q", res.Notes)
	}

	if err := s.SaveRecommendations(ctx, id, []model.Recommendation{{Entity: "EMAIL"}}); err != nil {
		t.Fatalf("SaveRecommendations: %v", err)
	}
	res, err = s.GetAuditResult(ctx, id)
	if err != nil {
		t.Fatalf("GetAuditResult: %v", err)
	}
	if len(res.Recommendations) != 1 || res.Recommendations[0].Entity != "EMAIL" {
		t.Errorf("recommendations were not replaced: %+v", res.Recommendations)
	}
}

func TestFileStorageDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStorage(t)

	id := saveOne(t, s, model.AuditResult{Timestamp: time.Now()})
	if err := s.DeleteAuditResult(ctx, id); err != nil {
		t.Fatalf("DeleteAuditResult: %v", err)
	}

	history, err := s.GetHistory(ctx)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected the audit to be gone, got %d entries", len(history))
	}
}

// Deleting an audit that is not there matches what MongoDB does for a delete
// that selects no documents: it succeeds.
func TestFileStorageDeleteMissingIsNoOp(t *testing.T) {
	s := newTestStorage(t)
	if err := s.DeleteAuditResult(context.Background(), "20260827_000000_deadbeef"); err != nil {
		t.Errorf("deleting an audit that does not exist should succeed, got %v", err)
	}
}

// Reading history must not create the directory, so running the UI somewhere
// with no audits leaves nothing behind.
func TestFileStorageDoesNotCreateDirUntilWritten(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "history")
	s := NewFileStorage(dir)

	history, err := s.GetHistory(ctx)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected no entries, got %d", len(history))
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("reading history created %s; it should only appear on a write", dir)
	}

	if err := s.SaveAuditResult(ctx, model.AuditResult{Timestamp: time.Now()}); err != nil {
		t.Fatalf("SaveAuditResult: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("expected the directory to exist after a write: %v", err)
	}
}

// An ID arrives from the ?id= query parameter, so it must never be able to
// reach a path outside the history directory.
func TestFileStorageRejectsTraversal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStorage(dir)

	outside := filepath.Join(dir, "..", "secret.json")
	if err := os.WriteFile(outside, []byte(`{"notes":"secret"}`), 0644); err != nil {
		t.Fatalf("writing the decoy: %v", err)
	}

	for _, id := range []string{
		"../secret",
		"../../etc/passwd",
		"..",
		"/etc/passwd",
		"a/b",
		`..\secret`,
		"",
	} {
		t.Run("id="+id, func(t *testing.T) {
			if _, err := s.GetAuditResult(ctx, id); err == nil {
				t.Errorf("GetAuditResult(%q) should have failed", id)
			}
			if err := s.DeleteAuditResult(ctx, id); err == nil {
				t.Errorf("DeleteAuditResult(%q) should have failed on a malformed ID", id)
			}
			if err := s.SaveAuditNotes(ctx, id, "x"); err == nil {
				t.Errorf("SaveAuditNotes(%q) should have failed", id)
			}
		})
	}

	if _, err := os.Stat(outside); err != nil {
		t.Errorf("the file outside the history directory was touched: %v", err)
	}
}

// History written before this storage existed is named for its timestamp alone
// and carries no id field. It should still be listed and readable.
func TestFileStorageReadsLegacyFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStorage(dir)

	legacy := model.AuditResult{
		Timestamp: time.Date(2026, 4, 15, 8, 11, 8, 0, time.UTC),
		F1Score:   0.91,
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "audit_20260415_081108.json"), data, 0644); err != nil {
		t.Fatalf("writing the legacy file: %v", err)
	}

	history, err := s.GetHistory(ctx)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected the legacy audit to be listed, got %d entries", len(history))
	}
	if history[0].ID != "20260415_081108" {
		t.Errorf("expected the ID to come from the filename, got %v", history[0].ID)
	}

	res, err := s.GetAuditResult(ctx, "20260415_081108")
	if err != nil {
		t.Fatalf("GetAuditResult: %v", err)
	}
	if res.F1Score != 0.91 {
		t.Errorf("unexpected legacy result: %+v", res)
	}
}

// Two audits finishing in the same second must not overwrite each other.
func TestFileStorageDistinctIDsWithinASecond(t *testing.T) {
	ctx := context.Background()
	s := newTestStorage(t)
	ts := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		if err := s.SaveAuditResult(ctx, model.AuditResult{Timestamp: ts, F1Score: float64(i)}); err != nil {
			t.Fatalf("SaveAuditResult: %v", err)
		}
	}

	history, err := s.GetHistory(ctx)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 5 {
		t.Fatalf("expected 5 distinct audits, got %d", len(history))
	}
}

// The temporary files used for atomic writes must not appear as history.
func TestFileStorageIgnoresUnrelatedFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStorage(dir)

	for _, name := range []string{"report.json", "notes.txt", "audit_bad.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("not an audit"), 0644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	history, err := s.GetHistory(ctx)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected unparseable and unrelated files to be skipped, got %d entries", len(history))
	}
}

// One entity can now carry both a recall gap and a precision warning, so
// resolving one must not touch the other. Matching on the entity, which is what
// the earlier version did, would mark both.
func TestFileStorageResolvesOneRecommendationOfSeveralForAnEntity(t *testing.T) {
	s := newTestStorage(t)

	id := saveOne(t, s, model.AuditResult{
		Timestamp: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Recommendations: []model.Recommendation{
			{ID: "recall_below_threshold:PERSON", Kind: model.KindRecallGap, Entity: "PERSON"},
			{ID: "precision_collapsed:PERSON", Kind: model.KindPrecisionCollapsed, Entity: "PERSON"},
		},
	})

	if err := s.ResolveRecommendation(context.Background(), id, "recall_below_threshold:PERSON"); err != nil {
		t.Fatalf("ResolveRecommendation: %v", err)
	}

	res, err := s.GetAuditResult(context.Background(), id)
	if err != nil {
		t.Fatalf("GetAuditResult: %v", err)
	}
	if !res.Recommendations[0].Resolved {
		t.Error("expected the recall recommendation to be resolved")
	}
	if res.Recommendations[1].Resolved {
		t.Error("resolving the recall recommendation must not resolve the precision warning")
	}
}

// Audits written before recommendations had IDs are still addressable by
// entity, which is all their stored recommendations carry.
func TestFileStorageFallsBackToEntityForLegacyRecommendations(t *testing.T) {
	s := newTestStorage(t)

	id := saveOne(t, s, model.AuditResult{
		Timestamp:       time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Recommendations: []model.Recommendation{{Entity: "SSN", Description: "add a filter"}},
	})

	if err := s.DismissRecommendation(context.Background(), id, "SSN"); err != nil {
		t.Fatalf("DismissRecommendation: %v", err)
	}

	res, err := s.GetAuditResult(context.Background(), id)
	if err != nil {
		t.Fatalf("GetAuditResult: %v", err)
	}
	if !res.Recommendations[0].Dismissed {
		t.Error("expected an ID-less recommendation to still be addressable by entity")
	}
}
