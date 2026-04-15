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

package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/philterd/philterscope/pkg/model"
)

//go:embed index.html
var staticAssets embed.FS

// StartServer launches the local Evaluation UI.
func StartServer(port int, result model.AuditResult) error {
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return err
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reportJSON, _ := json.Marshal(result)
		data := struct {
			ReportJSON template.JS
		}{
			ReportJSON: template.JS(reportJSON),
		}
		tmpl.Execute(w, data)
	})

	fmt.Printf("Evaluation UI available at http://localhost:%d\n\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// GenerateStandaloneReport creates a self-contained HTML file.
func GenerateStandaloneReport(result model.AuditResult) (string, error) {
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return "", err
	}

	reportJSON, _ := json.Marshal(result)
	data := struct {
		ReportJSON template.JS
	}{
		ReportJSON: template.JS(reportJSON),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
