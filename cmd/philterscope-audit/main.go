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
	threshold     float64
	thresholds    string
	groupName     string
	enableAI      bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "philterscope-audit",
		Short: "Audit redaction quality",
		RunE:  runAudit,
	}

	rootCmd.Flags().StringVar(&philterURL, "url", "http://localhost:8080", "Philter API URL")
	rootCmd.Flags().StringVar(&philterToken, "token", "", "Philter API Token")
	rootCmd.Flags().StringVar(&philterPolicy, "policy", "default", "Philter policy name")
	rootCmd.Flags().StringVar(&inputDir, "input", "./raw", "Directory of raw text files")
	rootCmd.Flags().StringVar(&goldenFile, "golden", "golden.json", "Golden dataset JSON file")
	rootCmd.Flags().StringVar(&outputDir, "output", ".", "Directory to export reports")
	rootCmd.Flags().Float64Var(&threshold, "threshold", 0.5, "Recall threshold for suggestions (0.0 to 1.0)")
	rootCmd.Flags().StringVar(&thresholds, "thresholds", "", "Per-entity recall thresholds (e.g., NAME=0.9,SSN=1.0)")
	rootCmd.Flags().StringVar(&groupName, "group", "default", "Assign a group name to the audit")
	rootCmd.Flags().BoolVar(&enableAI, "ai", false, "Enable AI-driven policy recommendations")

	rootCmd.CompletionOptions.DisableDefaultCmd = true

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

		var goldenSpans []model.Span
		var originalText string

		goldenPaths := []string{}

		if goldenFile != "" {
			info, err := os.Stat(goldenFile)
			if err == nil {
				if info.IsDir() {
					goldenPaths = append(goldenPaths, filepath.Join(goldenFile, f.Name()))
					if filepath.Ext(f.Name()) == ".json" {
						if strings.HasPrefix(f.Name(), "redacted") {
							goldenName := strings.Replace(f.Name(), "redacted", "golden", 1)
							goldenPaths = append(goldenPaths, filepath.Join(goldenFile, goldenName))
						}
					}
				} else {
					goldenPaths = append(goldenPaths, goldenFile)
				}
			}
		}

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
			originalText, goldenSpans = audit.ParseTaggedText(string(rawContent))
			rawContent = []byte(originalText)
		}

		var redacted string
		var actualSpans []model.Span

		if filepath.Ext(f.Name()) == ".json" {
			if r, s, err := audit.ParsePhilterExplain(rawContent); err == nil {
				redacted = r
				actualSpans = s
			}
		}

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
			Expected: originalText,
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

	if policyStr, err := client.GetPolicy(philterPolicy); err == nil {
		var policy map[string]any
		if err := json.Unmarshal([]byte(policyStr), &policy); err == nil {
			auditResult.Policy = policy
		}
	}

	suggester := suggest.NewBasicSuggester(threshold, entityThresholdMap)
	auditResult.Recommendations = suggester.Suggest(auditResult)

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

	jsonReportPath := filepath.Join(outputDir, "report.json")
	jsonReport, _ := json.MarshalIndent(auditResult, "", "  ")
	if err := os.WriteFile(jsonReportPath, jsonReport, 0644); err != nil {
		fmt.Printf("Warning: failed to write JSON report: %v\n", err)
	}

	if err := saveToHistory(cmd.Context(), auditResult); err != nil {
		fmt.Printf("Warning: failed to save to history: %v\n", err)
	}

	fmt.Printf("Audit complete. Precision: %.2f, Recall: %.2f, F1: %.2f\n", auditResult.Precision, auditResult.Recall, auditResult.F1Score)
	fmt.Printf("Reports exported to %s\n", outputDir)

	return nil
}

func saveToHistory(ctx context.Context, res model.AuditResult) error {
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
