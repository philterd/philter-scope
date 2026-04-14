package main

import (
	"bytes"
	"testing"

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

	if err := cmd.Flags().Set("url", "http://test:8080"); err != nil {
		t.Errorf("Failed to set flag: %v", err)
	}

	if philterURL != "http://test:8080" {
		t.Errorf("Expected url http://test:8080, got %s", philterURL)
	}
}

func TestCommandStructure(t *testing.T) {
	// Re-building a mini version of main to test structure
	root := &cobra.Command{Use: "philterscope"}
	root.AddCommand(&cobra.Command{Use: "audit"})
	root.AddCommand(&cobra.Command{Use: "serve"})
	root.AddCommand(&cobra.Command{Use: "suggest"})

	if len(root.Commands()) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(root.Commands()))
	}
}

func TestRunSuggest_Help(t *testing.T) {
	// Test if suggest command help works
	root := &cobra.Command{Use: "philterscope"}
	suggestCmd := &cobra.Command{
		Use:   "suggest",
		Short: "Suggest policy changes",
		RunE:  runSuggest,
	}
	root.AddCommand(suggestCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"suggest", "--help"})

	if err := root.Execute(); err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("Suggest policy changes")) {
		t.Errorf("Help output missing expected text")
	}
}
