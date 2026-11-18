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
	"os/exec"
	"strings"
	"testing"
)

// The audit CLI serves no HTTP, so it must not link internal/server, and with it the
// Prometheus client, again (#13). Version and the HTML report live in internal/report.
func TestAuditDoesNotLinkTheServer(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	for _, pkg := range strings.Fields(string(out)) {
		if pkg == "github.com/philterd/philterscope/internal/server" || strings.Contains(pkg, "prometheus") {
			t.Errorf("philterscope-audit depends on %s", pkg)
		}
	}
}
