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

package suggest

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestFilterKey(t *testing.T) {
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
		key, known := filterKey(c.entity)
		if key != c.key || known != c.known {
			t.Errorf("filterKey(%q) = (%q, %v), want (%q, %v)", c.entity, key, known, c.key, c.known)
		}
	}
}

// TestKnownIdentifiersMatchSchema keeps knownIdentifiers in step with the
// identifier keys of the policy schema Philter Scope targets. The schema in
// testdata is a copy of schema/1.3.0/schema.json from philterd/phisql.
func TestKnownIdentifiersMatchSchema(t *testing.T) {
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
		if _, ok := knownIdentifiers[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range knownIdentifiers {
		if _, ok := schema.Defs.Identifiers.Properties[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("schema identifiers missing from knownIdentifiers: %s", strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		t.Errorf("knownIdentifiers not in the schema: %s", strings.Join(extra, ", "))
	}
}

func parsed(t *testing.T, raw string) *policyView {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("bad policy: %v", err)
	}
	return parsePolicy(m)
}

func TestPolicyLookup(t *testing.T) {
	p := parsed(t, `{"identifiers": {"phoneNumber": {"enabled": true}, "ssn": {"enabled": false}}}`)

	if _, _, present := p.lookup("PHONE_NUMBER"); !present {
		t.Error("expected PHONE_NUMBER to resolve to the phoneNumber filter")
	}
	key, filter, present := p.lookup("SSN")
	if !present || key != "ssn" {
		t.Fatalf("expected the ssn filter, got key=%q present=%v", key, present)
	}
	if enabled(filter) {
		t.Error("expected the ssn filter to read as disabled")
	}
	if _, _, present := p.lookup("CREDIT_CARD"); present {
		t.Error("expected a filter the policy does not contain to be absent")
	}
}

// A filter present without "enabled" is on, which is how Philter defaults it.
func TestPolicyEnabledDefault(t *testing.T) {
	p := parsed(t, `{"identifiers": {"ssn": {}}}`)
	_, filter, present := p.lookup("SSN")
	if !present || !enabled(filter) {
		t.Error("a filter present without an enabled field should read as enabled")
	}
}

// A policy that could not be read reports unknown rather than absent.
func TestPolicyUnknown(t *testing.T) {
	if parsePolicy(nil).known() {
		t.Error("a nil policy is not a readable one")
	}
	if parsed(t, `{"name": "default"}`).known() {
		t.Error("a policy with no identifiers is not readable for this purpose")
	}
	if !parsed(t, `{"identifiers": {}}`).known() {
		t.Error("an empty identifiers block is still a readable policy")
	}
}

func TestConfidenceThreshold(t *testing.T) {
	p := parsed(t, `{"identifiers": {"person": {"enabled": true, "thresholds": {"PERSON": 0.4}}, "ssn": {"enabled": true}}}`)

	_, personFilter, _ := p.lookup("PERSON")
	value, supported := confidenceThreshold(personFilter, "PERSON")
	if !supported || value != 0.4 {
		t.Errorf("expected a supported threshold of 0.4, got %v supported=%v", value, supported)
	}

	// A filter with no thresholds block does not take one, which is what stops
	// a threshold change being proposed for it.
	_, ssnFilter, _ := p.lookup("SSN")
	if _, supported := confidenceThreshold(ssnFilter, "SSN"); supported {
		t.Error("expected a filter without a thresholds block to report no support")
	}
}

// A filter defined as a list, as pheyes is, still yields a body to read.
func TestPolicyLookupListFilter(t *testing.T) {
	p := parsed(t, `{"identifiers": {"pheyes": [{"enabled": true, "thresholds": {"PERSON": 0.8}}]}}`)
	key, filter, present := p.lookup("PH_EYE")
	if !present || key != "pheyes" || filter == nil {
		t.Fatalf("expected the pheyes filter body, got key=%q present=%v filter=%v", key, present, filter)
	}
	if v, supported := confidenceThreshold(filter, "PERSON"); !supported || v != 0.8 {
		t.Errorf("expected 0.8 from the list filter, got %v supported=%v", v, supported)
	}
}

// A gold standard label with no matching Philter filter must not produce a
// policy fragment, since any key written for it would be one Philter rejects.
func TestSuggest_UnmappableLabelProposesNoEdit(t *testing.T) {
	s := NewBasicSuggester(0.75, nil)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"PATIENT_NICKNAME": 0.1},
		EntityStats:   map[string]model.EntityStat{"PATIENT_NICKNAME": {TruePositives: 1, FalseNegatives: 9, Recall: 0.1}},
		Policy:        policyWith(t, `{"identifiers": {"ssn": {"enabled": true}}}`),
	}

	rec := findByKind(s.Suggest(result), model.KindRecallGap, "PATIENT_NICKNAME")
	if rec == nil {
		t.Fatal("expected a recall recommendation")
	}
	if rec.Change == nil || rec.Change.Type != model.ChangeReviewFilter {
		t.Fatalf("expected review_filter for an unmappable label, got %+v", rec.Change)
	}
	if rec.Change.Filter != "" {
		t.Errorf("expected no filter to be named, got %q", rec.Change.Filter)
	}
	if rec.Snippet != "" {
		t.Errorf("expected no snippet for an unmappable label, got:\n%s", rec.Snippet)
	}
}

func TestThresholdSnippet(t *testing.T) {
	got := thresholdSnippet("person", "PERSON", 0.75)
	want := `{
  "identifiers": {
    "person": {
      "thresholds": {
        "PERSON": 0.75
      }
    }
  }
}`
	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}
