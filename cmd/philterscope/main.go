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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/philterd/philterscope/internal/audit"
	"github.com/philterd/philterscope/internal/ollama"
	"github.com/philterd/philterscope/internal/philter"
	"github.com/philterd/philterscope/internal/server"
	"github.com/philterd/philterscope/internal/storage"
	"github.com/philterd/philterscope/internal/suggest"
	"github.com/philterd/philterscope/pkg/model"
	"github.com/spf13/cobra"
)

var (
	philterURL    string
	philterToken  string
	philterPolicy string
	inputDir      string
	goldenFile    string
	outputDir     string
	port          int
	threshold     float64
	thresholds    string
	groupName     string
	enableAI      bool
	privacy       bool
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
	auditCmd.Flags().StringVar(&philterPolicy, "policy", "default", "Philter policy name")
	auditCmd.Flags().StringVar(&inputDir, "input", "./raw", "Directory of raw text files")
	auditCmd.Flags().StringVar(&goldenFile, "golden", "golden.json", "Golden dataset JSON file")
	auditCmd.Flags().StringVar(&outputDir, "output", ".", "Directory to export reports")
	auditCmd.Flags().Float64Var(&threshold, "threshold", 0.5, "Recall threshold for suggestions (0.0 to 1.0)")
	auditCmd.Flags().StringVar(&thresholds, "thresholds", "", "Per-entity recall thresholds (e.g., NAME=0.9,SSN=1.0)")
	auditCmd.Flags().StringVar(&groupName, "group", "default", "Assign a group name to the audit")
	auditCmd.Flags().BoolVar(&enableAI, "ai", false, "Enable AI-driven policy recommendations")

	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Launch Evaluation UI",
		RunE:  runServe,
	}
	serveCmd.Flags().IntVar(&port, "port", 5000, "Port for the UI")
	serveCmd.Flags().StringVar(&goldenFile, "report", "report.json", "JSON report to serve")
	serveCmd.Flags().BoolVar(&privacy, "privacy", false, "Enable privacy mode (obfuscate PII in UI)")

	var historyCmd = &cobra.Command{
		Use:   "history",
		Short: "List past audits",
		RunE:  runHistory,
	}

	rootCmd.AddCommand(auditCmd, serveCmd, historyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAudit(cmd *cobra.Command, args []string) error {
	entityThresholdMap := make(map[string]float64)
	if thresholds != "" {
		pairs := strings.Split(thresholds, ",")
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			if len(kv) == 2 {
				var val float64
				if _, err := fmt.Sscanf(kv[1], "%f", &val); err == nil {
					entityThresholdMap[kv[0]] = val
				}
			}
		}
	}

	client := &philter.PhilterClient{
		BaseURL: philterURL,
		Token:   philterToken,
		Policy:  philterPolicy,
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
		// The requirement said "A simple text file" or "A JSON format"

		var goldenSpans []model.Span
		var originalText string

		// Try to find a matching golden file
		// 1. If --golden points to a single file, try it for all input files (if it contains labels for all, or matches filename)
		// 2. If --golden points to a directory, look for matches there
		// 3. Check for <filename>.golden in inputDir
		// 4. Check for golden/<filename>
		goldenPaths := []string{}

		if goldenFile != "" {
			info, err := os.Stat(goldenFile)
			if err == nil {
				if info.IsDir() {
					// It's a directory, look for matching filenames
					goldenPaths = append(goldenPaths, filepath.Join(goldenFile, f.Name()))
					if filepath.Ext(f.Name()) == ".json" {
						if strings.HasPrefix(f.Name(), "redacted") {
							goldenName := strings.Replace(f.Name(), "redacted", "golden", 1)
							goldenPaths = append(goldenPaths, filepath.Join(goldenFile, goldenName))
						}
					}
				} else {
					// It's a single file
					goldenPaths = append(goldenPaths, goldenFile)
				}
			}
		}

		// Always add traditional fallbacks
		goldenPaths = append(goldenPaths,
			filepath.Join(inputDir, f.Name()+".golden"),
			filepath.Join(filepath.Dir(inputDir), "golden", f.Name()),
		)
		if filepath.Ext(f.Name()) == ".json" {
			goldenPaths = append(goldenPaths, filepath.Join(filepath.Dir(inputDir), "golden", f.Name()))
			if strings.HasPrefix(f.Name(), "redacted") {
				goldenName := strings.Replace(f.Name(), "redacted", "golden", 1)
				goldenPaths = append(goldenPaths, filepath.Join(filepath.Dir(inputDir), "golden", goldenName))
			}
		}

		foundGolden := false
		for _, gp := range goldenPaths {
			if _, err := os.Stat(gp); err == nil {
				gContent, _ := os.ReadFile(gp)
				if filepath.Ext(gp) == ".json" {
					originalText, goldenSpans, err = audit.ParseJSONSpans(gContent)
					if err == nil {
						foundGolden = true
						break
					}
				} else {
					originalText, goldenSpans = audit.ParseTaggedText(string(gContent))
					foundGolden = true
					break
				}
			}
		}

		if !foundGolden {
			// Fallback: check if the input file itself is tagged (self-labeled)
			originalText, goldenSpans = audit.ParseTaggedText(string(rawContent))
			// Use the clean text for Philter
			rawContent = []byte(originalText)
		}

		var redacted string
		var actualSpans []model.Span

		// If the input file is a JSON file, it might be a Philter explain response
		if filepath.Ext(f.Name()) == ".json" {
			if r, s, err := audit.ParsePhilterExplain(rawContent); err == nil {
				redacted = r
				actualSpans = s
			}
		}

		// If not already set (not a Philter explain JSON), call Philter API
		if redacted == "" && len(actualSpans) == 0 {
			redacted, actualSpans, err = client.Redact(string(rawContent))
			if err != nil {
				fmt.Printf("Warning: failed to redact %s: %v\n", f.Name(), err)
				continue
			}
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
	auditResult.Threshold = threshold
	auditResult.EntityThresholds = entityThresholdMap
	auditResult.GroupName = groupName

	// Try to get policy from Philter
	if policyStr, err := client.GetPolicy(philterPolicy); err == nil {
		var policy map[string]any
		if err := json.Unmarshal([]byte(policyStr), &policy); err == nil {
			auditResult.Policy = policy
		}
	}

	// Generate suggestions
	suggester := suggest.NewBasicSuggester(threshold, entityThresholdMap)
	auditResult.Recommendations = suggester.Suggest(auditResult)

	// AI recommendations if enabled
	if enableAI {
		if os.Getenv("PHILTERSCOPE_OLLAMA_URL") != "" {
			fmt.Println("Generating AI recommendations...")
			client := ollama.NewClient()
			ls := suggest.NewLLMSuggester(client)
			aiRecs := ls.Suggest(auditResult)
			for i := range aiRecs {
				aiRecs[i].Description = "[AI] " + aiRecs[i].Description
			}
			auditResult.Recommendations = append(auditResult.Recommendations, aiRecs...)
		} else {
			fmt.Println("Warning: AI suggestions requested but PHILTERSCOPE_OLLAMA_URL is not set.")
		}
	}

	// Export results
	htmlReport, err := server.GenerateStandaloneReport(auditResult, false)
	if err != nil {
		return fmt.Errorf("failed to generate HTML report: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	htmlPath := filepath.Join(outputDir, "report.html")
	if err := os.WriteFile(htmlPath, []byte(htmlReport), 0644); err != nil {
		return fmt.Errorf("failed to write HTML report: %w", err)
	}

	// Also save as JSON for serve/suggest commands
	jsonReportPath := filepath.Join(outputDir, "report.json")
	jsonReport, _ := json.MarshalIndent(auditResult, "", "  ")
	if err := os.WriteFile(jsonReportPath, jsonReport, 0644); err != nil {
		fmt.Printf("Warning: failed to write JSON report: %v\n", err)
	}

	// Policy Versioning: Save snapshot to .philterscope folder or MongoDB
	if err := saveToHistory(cmd.Context(), auditResult); err != nil {
		fmt.Printf("Warning: failed to save to history: %v\n", err)
	}

	fmt.Printf("Audit complete. Precision: %.2f, Recall: %.2f, F1: %.2f\n", auditResult.Precision, auditResult.Recall, auditResult.F1Score)
	fmt.Printf("Reports exported to %s\n", outputDir)

	return nil
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	entityThresholdMap := make(map[string]float64)
	if thresholds != "" {
		pairs := strings.Split(thresholds, ",")
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			if len(kv) == 2 {
				var val float64
				if _, err := fmt.Sscanf(kv[1], "%f", &val); err == nil {
					entityThresholdMap[kv[0]] = val
				}
			}
		}
	}

	// Try MongoDB if configured
	var mongoErr error
	if os.Getenv("PHILTERSCOPE_MONGODB_CONNECTION_STRING") != "" {
		m, err := storage.NewMongoDBStorage(ctx)
		if err == nil {
			defer m.Close(ctx)
			return server.StartServer(port, m, privacy)
		}
		mongoErr = err
		fmt.Printf("Warning: failed to connect to MongoDB: %v. Falling back to file mode.\n", err)
	}

	// Fallback to reading from a file if goldenFile is set
	if goldenFile != "" {
		data, err := os.ReadFile(goldenFile)
		if err != nil {
			return err
		}
		var res model.AuditResult
		if err := json.Unmarshal(data, &res); err != nil {
			return err
		}
		res.Threshold = threshold
		res.EntityThresholds = entityThresholdMap
		if mongoErr != nil {
			res.Notes = fmt.Sprintf("Warning: Failed to connect to MongoDB: %v. %s", mongoErr, res.Notes)
		}
		return server.StartStandaloneServer(port, res, privacy)
	}

	return fmt.Errorf("no MongoDB connection string (PHILTERSCOPE_MONGODB_CONNECTION_STRING) and no input file (--golden) provided")
}

func saveToHistory(ctx context.Context, res model.AuditResult) error {
	// Try MongoDB first if configured
	if os.Getenv("PHILTERSCOPE_MONGODB_CONNECTION_STRING") != "" {
		m, err := storage.NewMongoDBStorage(ctx)
		if err == nil {
			defer m.Close(ctx)
			if err := m.SaveAuditResult(ctx, res); err == nil {
				return nil
			} else {
				fmt.Printf("Warning: failed to save to MongoDB: %v\n", err)
			}
		} else {
			fmt.Printf("Warning: failed to connect to MongoDB: %v\n", err)
		}
	}

	// Fallback to local storage
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
	var history []model.HistoryEntry
	ctx := cmd.Context()

	// Try MongoDB first if configured
	if os.Getenv("PHILTERSCOPE_MONGODB_CONNECTION_STRING") != "" {
		m, err := storage.NewMongoDBStorage(ctx)
		if err == nil {
			defer m.Close(ctx)
			h, err := m.GetHistory(ctx)
			if err == nil {
				history = h
			} else {
				fmt.Printf("Warning: failed to fetch history from MongoDB: %v\n", err)
			}
		} else {
			fmt.Printf("Warning: failed to connect to MongoDB: %v\n", err)
		}
	}

	// If MongoDB didn't provide history (or not configured), try local storage
	if len(history) == 0 {
		historyDir := ".philterscope"
		if _, err := os.Stat(historyDir); os.IsNotExist(err) {
			fmt.Println("No audit history found.")
			return nil
		}

		files, err := os.ReadDir(historyDir)
		if err != nil {
			return err
		}

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
				Threshold: res.Threshold,
				Policy:    res.Policy,
			})
		}
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
