package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/philterd/philterscope/internal/audit"
	"github.com/philterd/philterscope/internal/philter"
	"github.com/philterd/philterscope/internal/server"
	"github.com/philterd/philterscope/internal/suggest"
	"github.com/philterd/philterscope/pkg/model"
	"github.com/spf13/cobra"
)

var (
	philterURL   string
	philterToken string
	inputDir     string
	goldenFile   string
	outputFile   string
	port         int
)

func main() {
	var rootCmd = &cobra.Command{Use: "philterscope"}

	var auditCmd = &cobra.Command{
		Use:   "audit",
		Short: "Audit redaction quality",
		RunE:  runAudit,
	}

	auditCmd.Flags().StringVar(&philterURL, "url", "http://localhost:8080", "Philter API URL")
	auditCmd.Flags().StringVar(&philterToken, "token", "", "Philter API Token")
	auditCmd.Flags().StringVar(&inputDir, "input", "./raw", "Directory of raw text files")
	auditCmd.Flags().StringVar(&goldenFile, "golden", "golden.json", "Golden dataset JSON file")
	auditCmd.Flags().StringVar(&outputFile, "output", "report.html", "Path to export HTML report")

	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Launch Privacy Lab UI",
		RunE:  runServe,
	}
	serveCmd.Flags().IntVar(&port, "port", 5000, "Port for the UI")
	serveCmd.Flags().StringVar(&goldenFile, "report", "report.json", "JSON report to serve")

	var suggestCmd = &cobra.Command{
		Use:   "suggest",
		Short: "Suggest policy changes",
		RunE:  runSuggest,
	}
	suggestCmd.Flags().StringVar(&goldenFile, "report", "report.json", "JSON report to analyze")

	rootCmd.AddCommand(auditCmd, serveCmd, suggestCmd)

	var historyCmd = &cobra.Command{
		Use:   "history",
		Short: "List past audits",
		RunE:  runHistory,
	}

	rootCmd.AddCommand(auditCmd, serveCmd, suggestCmd, historyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAudit(cmd *cobra.Command, args []string) error {
	client := &philter.PhilterClient{
		BaseURL: philterURL,
		Token:   philterToken,
	}

	files, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	var results []model.Result
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		rawPath := filepath.Join(inputDir, f.Name())
		rawContent, err := os.ReadFile(rawPath)
		if err != nil {
			fmt.Printf("Warning: failed to read %s: %v\n", f.Name(), err)
			continue
		}

		// Determine golden format and parse
		// We expect golden file to be in a specific format or in a 'golden' directory
		// Let's assume for now golden file is either a single JSON file or we look for <filename>.golden in inputDir
		// The requirement said "A simple text file" or "A JSON format"

		var goldenSpans []model.Span
		var originalText string

		goldenPath := filepath.Join(inputDir, f.Name()+".golden")
		if _, err := os.Stat(goldenPath); err == nil {
			// Found a .golden file for this raw file
			gContent, _ := os.ReadFile(goldenPath)
			if filepath.Ext(goldenPath) == ".json" {
				originalText, goldenSpans, _ = audit.ParseJSONSpans(gContent)
			} else {
				originalText, goldenSpans = audit.ParseTaggedText(string(gContent))
			}
		} else {
			// Fallback: check if the input file itself is tagged (self-labeled)
			originalText, goldenSpans = audit.ParseTaggedText(string(rawContent))
			// Use the clean text for Philter
			rawContent = []byte(originalText)
		}

		redacted, actualSpans, err := client.Redact(string(rawContent))
		if err != nil {
			fmt.Printf("Warning: failed to redact %s: %v\n", f.Name(), err)
			continue
		}

		overlaps := audit.CalculateOverlap(goldenSpans, actualSpans)
		tp, fp, fn := audit.CalculateMetricsByOverlap(overlaps)

		results = append(results, model.Result{
			Filename: f.Name(),
			Expected: originalText, // Not exactly 'expected' anymore, but the clean version
			Actual:   redacted,
			Spans:    actualSpans,
			TP:       tp,
			FP:       fp,
			FN:       fn,
			Overlaps: overlaps,
		})
	}

	auditResult := audit.GenerateAuditResult(results)
	auditResult.Timestamp = time.Now()

	// Try to get policy from Philter
	if policy, err := client.GetPolicy(); err == nil {
		auditResult.Policy = policy
	}

	// Generate suggestions
	suggester := suggest.NewBasicSuggester(0.5)
	auditResult.Recommendations = suggester.Suggest(auditResult)

	// Export results
	htmlReport, err := server.GenerateStandaloneReport(auditResult)
	if err != nil {
		return fmt.Errorf("failed to generate HTML report: %w", err)
	}

	if err := os.WriteFile(outputFile, []byte(htmlReport), 0644); err != nil {
		return fmt.Errorf("failed to write HTML report: %w", err)
	}

	// Also save as JSON for serve/suggest commands
	jsonReport, _ := json.MarshalIndent(auditResult, "", "  ")
	os.WriteFile("report.json", jsonReport, 0644)

	// Policy Versioning: Save snapshot to .philterscope folder
	if err := saveToHistory(auditResult); err != nil {
		fmt.Printf("Warning: failed to save to history: %v\n", err)
	}

	fmt.Printf("Audit complete. Precision: %.2f, Recall: %.2f, F1: %.2f\n", auditResult.Precision, auditResult.Recall, auditResult.F1Score)
	fmt.Printf("HTML report exported to %s\n", outputFile)

	return nil
}

func runServe(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(goldenFile)
	if err != nil {
		return err
	}
	var res model.AuditResult
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	return server.StartServer(port, res)
}

func runSuggest(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(goldenFile)
	if err != nil {
		return err
	}
	var res model.AuditResult
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	suggest.GetSuggestions(res)
	return nil
}

func saveToHistory(res model.AuditResult) error {
	historyDir := ".philterscope"
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		if err := os.Mkdir(historyDir, 0755); err != nil {
			return err
		}
	}

	filename := fmt.Sprintf("audit_%s.json", res.Timestamp.Format("20060102_150405"))
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(historyDir, filename), data, 0644)
}

func runHistory(cmd *cobra.Command, args []string) error {
	historyDir := ".philterscope"
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		fmt.Println("No audit history found.")
		return nil
	}

	files, err := os.ReadDir(historyDir)
	if err != nil {
		return err
	}

	var history []model.HistoryEntry
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(historyDir, f.Name()))
		if err != nil {
			continue
		}

		var res model.AuditResult
		if err := json.Unmarshal(data, &res); err != nil {
			continue
		}

		history = append(history, model.HistoryEntry{
			Timestamp: res.Timestamp,
			Precision: res.Precision,
			Recall:    res.Recall,
			F1Score:   res.F1Score,
			Policy:    res.Policy,
		})
	}

	if len(history) == 0 {
		fmt.Println("No audit history found.")
		return nil
	}

	// Sort history by timestamp
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})

	fmt.Println("Audit History:")
	fmt.Printf("%-20s | %-10s | %-10s | %-10s\n", "Timestamp", "Precision", "Recall", "F1")
	fmt.Println("---------------------------------------------------------------")

	var lastF1 float64
	for i, entry := range history {
		f1 := entry.F1Score
		trend := ""
		if i > 0 {
			if f1 > lastF1 {
				trend = " (Improving ↑)"
			} else if f1 < lastF1 {
				trend = " (Declining ↓)"
			} else {
				trend = " (Steady =)"
			}
		}

		fmt.Printf("%-20s | %-10.2f | %-10.2f | %-10.2f%s\n",
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.Precision,
			entry.Recall,
			entry.F1Score,
			trend)
		lastF1 = f1
	}

	return nil
}
