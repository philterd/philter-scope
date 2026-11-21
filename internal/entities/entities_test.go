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

package entities

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestKey(t *testing.T) {
	cases := []struct {
		entity string
		key    string
		known  bool
	}{
		{"PHONE_NUMBER", "phoneNumber", true},
		{"phone-number", "phoneNumber", true},
		{"phoneNumber", "phoneNumber", true},
		{"SSN", "ssn", true},
		{"CREDIT_CARD", "creditCard", true},
		{"EMAIL_ADDRESS", "emailAddress", true},
		{"LOCATION_CITY", "city", true},
		{"DRIVERS_LICENSE_NUMBER", "driversLicense", true},
		// Short labels a gold standard tends to use.
		{"NAME", "person", true},
		{"ADDRESS", "streetAddress", true},
		{"EMAIL", "emailAddress", true},
		// Filters the schema gained after the list was written.
		{"EIN", "ein", true},
		{"ITIN", "itin", true},
		{"CANADA_SIN", "canadaSin", true},
		{"NPI", "npi", true},
		{"DEA_NUMBER", "deaNumber", true},
		{"dea-number", "deaNumber", true},
		{"MBI", "mbi", true},
		// Schema 1.3.0 has no physicianName filter.
		{"PHYSICIAN_NAME", "physicianName", false},
		// A label naming no Philter filter must say so rather than invent one.
		{"PATIENT_NICKNAME", "patientNickname", false},
		{"LOCATION", "location", false},
	}

	for _, c := range cases {
		key, known := Key(c.entity)
		if key != c.key || known != c.known {
			t.Errorf("Key(%q) = (%q, %v), want (%q, %v)", c.entity, key, known, c.key, c.known)
		}
	}
}

// TestKnownKeysMatchSchema keeps knownKeys in step with the
// identifier keys of the policy schema Philter Scope targets. The schema in
// testdata is a copy of schema/1.3.0/schema.json from philterd/phisql.
func TestKnownKeysMatchSchema(t *testing.T) {
	raw, err := os.ReadFile("testdata/phisql-schema-1.3.0.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Defs struct {
			Identifiers struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"identifiers"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if len(schema.Defs.Identifiers.Properties) == 0 {
		t.Fatal("found no identifier keys in the schema")
	}

	var missing, extra []string
	for key := range schema.Defs.Identifiers.Properties {
		if _, ok := knownKeys[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range knownKeys {
		if _, ok := schema.Defs.Identifiers.Properties[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("schema identifiers missing from knownKeys: %s", strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		t.Errorf("knownKeys not in the schema: %s", strings.Join(extra, ", "))
	}
}

func TestName(t *testing.T) {
	cases := map[string]string{
		"NAME":          "person",
		"PERSON":        "person",
		"EMAIL":         "emailAddress",
		"EMAIL_ADDRESS": "emailAddress",
		"PHONE":         "phoneNumber",
		"PHONE_NUMBER":  "phoneNumber",
		"LOCATION_CITY": "city",
		// A label that names no filter keeps its own spelling.
		"LOCATION":         "LOCATION",
		"PATIENT_NICKNAME": "PATIENT_NICKNAME",
		"":                 "",
	}
	for label, want := range cases {
		if got := Name(label); got != want {
			t.Errorf("Name(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestOf(t *testing.T) {
	cases := []struct {
		span model.Span
		want string
	}{
		{model.Span{Label: "NAME", FilterType: "NAME"}, "person"},
		{model.Span{FilterType: "PERSON"}, "person"},
		// The label wins over the filter type.
		{model.Span{Label: "EMAIL", FilterType: "PHONE_NUMBER"}, "emailAddress"},
		{model.Span{Label: "LOCATION"}, "LOCATION"},
		{model.Span{}, ""},
	}
	for _, c := range cases {
		if got := Of(c.span); got != c.want {
			t.Errorf("Of(%+v) = %q, want %q", c.span, got, c.want)
		}
	}
}
