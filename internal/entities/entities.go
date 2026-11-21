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

// Package entities names the entities an audit reports on. A gold standard and
// Philter often spell one filter differently, such as NAME and PERSON, or EMAIL
// and EMAIL_ADDRESS, so both are reported under the identifier key a policy
// uses for that filter.
package entities

import (
	"strings"

	"github.com/philterd/philterscope/pkg/model"
)

// knownKeys is the set of filter keys a Philter policy can contain.
//
// It exists so a gold standard label that maps to no Philter filter produces
// "I cannot tell you which filter this is" rather than a policy fragment keyed
// by something Philter would reject. A policy naming a filter this list has not
// caught up with is still matched, since lookup searches the policy itself.
//
// The keys are those of "identifiers" in the redaction policy schema 1.3.0
// (philterd/phisql). TestKnownKeysMatchSchema compares the two, so a
// filter added to the schema, or taken out of it, fails the test here until
// this list follows.
var knownKeys = map[string]struct{}{
	"pheyes": {}, "person": {}, "dictionaries": {}, "age": {}, "bankRoutingNumber": {},
	"bitcoinAddress": {}, "canadaSin": {}, "creditCard": {}, "currency": {}, "date": {},
	"deaNumber": {}, "driversLicense": {}, "ein": {}, "emailAddress": {}, "ibanCode": {},
	"identifiers": {}, "ipAddress": {}, "itin": {}, "macAddress": {}, "mbi": {}, "npi": {},
	"passportNumber": {}, "phoneNumber": {}, "phoneNumberExtension": {}, "sections": {},
	"ssn": {}, "stateAbbreviation": {}, "streetAddress": {}, "trackingNumber": {}, "url": {},
	"vin": {}, "zipCode": {}, "medicalCondition": {}, "city": {}, "county": {},
	"firstName": {}, "hospital": {}, "state": {}, "surname": {},
}

// entityAliases maps entity labels whose camelCase form is not the policy's
// identifier key. Two kinds of entry live here: Philter filter types that are
// spelled differently in a policy, and the short labels gold standards commonly
// use for them. The mapping is a convenience, and an unmapped label falls
// through to "unknown filter" rather than to a guess.
var entityAliases = map[string]string{
	// Philter filter types whose policy key differs from the type name.
	"locationcity":         "city",
	"locationstate":        "state",
	"locationcounty":       "county",
	"driverslicensenumber": "driversLicense",
	"pheye":                "pheyes",
	"customdictionary":     "dictionaries",
	"identifier":           "identifiers",
	"section":              "sections",
	"hospitalabbreviation": "hospital",

	// Labels a gold standard commonly uses for those filters.
	"name":     "person",
	"fullname": "person",
	"address":  "streetAddress",
	"email":    "emailAddress",
	"phone":    "phoneNumber",
	"zip":      "zipCode",
	"zipcode":  "zipCode",
}

// Normalize reduces a label to letters and digits in lower case, so the
// several spellings of one filter compare equal.
func Normalize(entity string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(entity) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// camelize turns an entity label into the lowerCamelCase spelling a Philter
// policy uses for its identifier keys, so PHONE_NUMBER, phone-number, and
// phoneNumber all render as phoneNumber. A label that is already one word
// comes back lowercased, which is how the single-word keys are spelled.
func camelize(entity string) string {
	words := strings.FieldsFunc(entity, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})

	var b strings.Builder
	for i, w := range words {
		lower := strings.ToLower(w)
		if i == 0 {
			b.WriteString(lower)
			continue
		}
		b.WriteString(strings.ToUpper(lower[:1]))
		b.WriteString(lower[1:])
	}
	return b.String()
}

// Key returns the identifier key a policy would use for an entity, and
// whether that key names a filter Philter actually has. A key produced here can
// be written into a snippet, so it has to be spelled the way a policy spells it
// rather than merely normalized for comparison.
//
// Gold standard labels are free text. One that names no Philter filter reports
// known as false, and the caller then says so instead of proposing a change
// against a filter that does not exist.
func Key(entity string) (string, bool) {
	norm := Normalize(entity)
	if alias, ok := entityAliases[norm]; ok {
		return alias, true
	}
	// Matching on the normalized form catches every spelling of a known filter,
	// so PHONE_NUMBER, phone-number, and phoneNumber all land on phoneNumber
	// without camelize having to reconstruct the word boundaries.
	if canonical, ok := knownByNormalized[norm]; ok {
		return canonical, true
	}
	return camelize(entity), false
}

// knownByNormalized indexes the known filter keys by their normalized form.
var knownByNormalized = func() map[string]string {
	index := make(map[string]string, len(knownKeys))
	for key := range knownKeys {
		index[Normalize(key)] = key
	}
	return index
}()

// Name returns the name an audit reports an entity label under: the policy's
// identifier key when the label names a Philter filter, and the label itself
// when it does not, so a label like LOCATION is not renamed to a guess.
func Name(label string) string {
	if key, known := Key(label); known {
		return key
	}
	return label
}

// Of returns the name of the entity a span belongs to. The span's label is
// preferred and its filter type is the fallback, so a span labeled either way
// resolves to the same entity. A span with neither returns "".
func Of(s model.Span) string {
	if s.Label != "" {
		return Name(s.Label)
	}
	if s.FilterType != "" {
		return Name(s.FilterType)
	}
	return ""
}
