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

package report

// Version is the application version, set at build time with
// -ldflags "-X github.com/philterd/philterscope/internal/report.Version=...".
// Builds made outside the Makefile (go build, go test, an IDE) report "dev".
// It lives here rather than in internal/server so philterscope-audit, which
// serves no HTTP, can report it without linking the server and its metrics.
var Version = "dev"
