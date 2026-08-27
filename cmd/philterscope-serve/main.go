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
	var rootCmd = &cobra.Command{
		Use:     "philterscope-serve",
		Short:   "Launch Evaluation UI",
		Version: server.Version,
		RunE:    runServe,
	}

	rootCmd.Flags().IntVar(&port, "port", 5000, "Port for the UI")
	rootCmd.Flags().StringVar(&goldenFile, "report", "report.json", "JSON report to serve")
	rootCmd.Flags().BoolVar(&privacy, "privacy", false, "Enable privacy mode (obfuscate PII in UI)")
	rootCmd.Flags().Float64Var(&threshold, "threshold", 0.5, "Recall threshold for suggestions (0.0 to 1.0)")
	rootCmd.Flags().StringVar(&thresholds, "thresholds", "", "Per-entity recall thresholds (e.g., NAME=0.9,SSN=1.0)")

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	// The error is printed once, below, rather than by cobra as well.
	rootCmd.SilenceErrors = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	// The flags parsed, so anything that fails from here is a failure to run,
	// not a usage error, and should not be answered with the whole flag list.
	cmd.SilenceUsage = true

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

	// Serve a single report when one was asked for explicitly, or when the
	// default report.json is sitting there to be served.
	_, statErr := os.Stat(goldenFile)
	if goldenFile != "" && (cmd.Flags().Changed("report") || statErr == nil) {
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

	// Otherwise serve the local audit history, so audits saved without MongoDB
	// are browsable rather than write-only.
	fs := storage.NewFileStorage("")
	fmt.Printf("Serving audit history from %s\n", fs.Dir())
	return server.StartServer(port, fs, privacy)
}
