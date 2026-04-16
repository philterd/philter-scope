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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/philterd/philterscope/internal/server"
	"github.com/philterd/philterscope/internal/storage"
	"github.com/philterd/philterscope/pkg/model"
	"github.com/spf13/cobra"
)

var (
	goldenFile string
	port       int
	threshold  float64
	thresholds string
	privacy    bool
)

func main() {
	var rootCmd = &cobra.Command{Use: "philterscope-serve"}

	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Launch Evaluation UI",
		RunE:  runServe,
	}
	serveCmd.Flags().IntVar(&port, "port", 5000, "Port for the UI")
	serveCmd.Flags().StringVar(&goldenFile, "report", "report.json", "JSON report to serve")
	serveCmd.Flags().BoolVar(&privacy, "privacy", false, "Enable privacy mode (obfuscate PII in UI)")
	serveCmd.Flags().Float64Var(&threshold, "threshold", 0.5, "Recall threshold for suggestions (0.0 to 1.0)")
	serveCmd.Flags().StringVar(&thresholds, "thresholds", "", "Per-entity recall thresholds (e.g., NAME=0.9,SSN=1.0)")

	var historyCmd = &cobra.Command{
		Use:   "history",
		Short: "List past audits",
		RunE:  runHistory,
	}

	rootCmd.AddCommand(serveCmd, historyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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

	return fmt.Errorf("no MongoDB connection string (PHILTERSCOPE_MONGODB_CONNECTION_STRING) and no input file (--report) provided")
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
