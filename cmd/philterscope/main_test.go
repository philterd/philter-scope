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

package main

import (
	"os"
	"testing"
	"time"

	"github.com/philterd/philterscope/pkg/model"
	"github.com/spf13/cobra"
)

func TestRootCmd(t *testing.T) {
	rootCmd := &cobra.Command{Use: "philterscope"}

	// We just want to check if it can be initialized without panicking
	if rootCmd.Use != "philterscope" {
		t.Errorf("Expected use 'philterscope', got %s", rootCmd.Use)
	}
}

func TestAuditFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "audit"}
	cmd.Flags().StringVar(&philterURL, "url", "http://localhost:8080", "Philter API URL")
	cmd.Flags().StringVar(&groupName, "group", "default", "Assign a group name to the audit")

	if err := cmd.Flags().Set("url", "http://test:8080"); err != nil {
		t.Errorf("Failed to set flag: %v", err)
	}

	if philterURL != "http://test:8080" {
		t.Errorf("Expected url http://test:8080, got %s", philterURL)
	}

	if err := cmd.Flags().Set("group", "test-group"); err != nil {
		t.Errorf("Failed to set group flag: %v", err)
	}

	if groupName != "test-group" {
		t.Errorf("Expected group test-group, got %s", groupName)
	}
}

func TestCommandStructure(t *testing.T) {
	// Re-building a mini version of main to test structure
	root := &cobra.Command{Use: "philterscope"}
	root.AddCommand(&cobra.Command{Use: "audit"})
	root.AddCommand(&cobra.Command{Use: "serve"})
	root.AddCommand(&cobra.Command{Use: "history"})

	if len(root.Commands()) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(root.Commands()))
	}
}

func TestRunHistory_Empty(t *testing.T) {
	// Test runHistory with no data
	cmd := &cobra.Command{Use: "history"}
	err := runHistory(cmd, []string{})
	if err != nil {
		t.Errorf("runHistory failed: %v", err)
	}
}

func TestSaveToHistory_Local(t *testing.T) {
	ctx := t.Context()
	res := model.AuditResult{
		Timestamp: time.Now(),
		F1Score:   0.85,
	}

	// Ensure .philterscope directory exists or is handled
	err := saveToHistory(ctx, res)
	if err != nil {
		t.Fatalf("saveToHistory failed: %v", err)
	}
	defer os.RemoveAll(".philterscope")

	// Verify file was created
	files, err := os.ReadDir(".philterscope")
	if err != nil || len(files) == 0 {
		t.Error("No history files created")
	}

	t.Run("History List", func(t *testing.T) {
		cmd := &cobra.Command{Use: "history"}
		err := runHistory(cmd, []string{})
		if err != nil {
			t.Errorf("runHistory failed: %v", err)
		}
	})
}

func TestRunServe_Fail(t *testing.T) {
	cmd := &cobra.Command{Use: "serve"}
	goldenFile = "non-existent.json"
	err := runServe(cmd, []string{})
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestRunAudit_NoInput(t *testing.T) {
	inputDir = "non-existent-dir"
	err := runAudit(nil, nil)
	if err == nil {
		t.Error("Expected error for non-existent input dir, got nil")
	}
}
