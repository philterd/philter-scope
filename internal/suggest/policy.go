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
	"fmt"
	"strings"

	"github.com/philterd/philterscope/internal/entities"
)

// policyView is a read-only view over the Philter policy an audit ran against.
//
// A Philter policy nests its filters under "identifiers", keyed by a camelCase
// name ("phoneNumber", "creditCard"), and each filter carries "enabled". The
// model-backed filters also carry a "thresholds" map of label to confidence
// cutoff. The policy is read as generic JSON rather than typed structs, since
// the schema belongs to Philter and this only needs a few fields from it: a
// shape it does not recognize degrades to "no filter found", which costs a
// suggestion some precision but never invents a wrong one.
type policyView struct {
	identifiers map[string]any
}

// parsePolicy reads the identifiers out of an audit's policy. A nil or
// unrecognized policy yields a view that reports every filter as absent, which
// is the same answer as having no policy at all.
func parsePolicy(policy map[string]any) *policyView {
	v := &policyView{}
	if policy == nil {
		return v
	}
	if ids, ok := policy["identifiers"].(map[string]any); ok {
		v.identifiers = ids
	}
	return v
}

// known reports whether the policy was readable at all. When it was not, a
// suggestion must not claim a filter is missing, only that it could not tell.
func (p *policyView) known() bool {
	return p.identifiers != nil
}

// lookup finds the filter for an entity. It returns the identifier key as the
// policy spells it, the filter body, and whether the policy contains it. A
// filter defined as a list, as "pheyes" is, yields its first element, which is
// enough to read "enabled" and "thresholds" from.
func (p *policyView) lookup(entity string) (string, map[string]any, bool) {
	key, _ := entities.Key(entity)
	if p.identifiers == nil {
		return key, nil, false
	}

	norm := entities.Normalize(key)
	for name, body := range p.identifiers {
		if entities.Normalize(name) != norm {
			continue
		}
		switch f := body.(type) {
		case map[string]any:
			return name, f, true
		case []any:
			for _, item := range f {
				if m, ok := item.(map[string]any); ok {
					return name, m, true
				}
			}
			return name, nil, true
		}
	}

	return key, nil, false
}

// enabled reports whether a filter body is switched on. Philter defaults
// "enabled" to true, so a filter that is present without the field is on.
func enabled(filter map[string]any) bool {
	if filter == nil {
		return true
	}
	if v, ok := filter["enabled"].(bool); ok {
		return v
	}
	return true
}

// confidenceThreshold reads the cutoff a filter applies to an entity, when the
// filter is one that has thresholds at all. The second return says whether the
// filter supports a threshold, which is what decides if a threshold change can
// be suggested for it.
func confidenceThreshold(filter map[string]any, entity string) (float64, bool) {
	if filter == nil {
		return 0, false
	}
	raw, ok := filter["thresholds"].(map[string]any)
	if !ok {
		return 0, false
	}

	norm := entities.Normalize(entity)
	for label, value := range raw {
		if entities.Normalize(label) != norm {
			continue
		}
		if f, ok := toFloat(value); ok {
			return f, true
		}
	}

	// The filter takes thresholds but has none set for this entity.
	return 0, true
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// enableSnippet is the policy fragment that turns a filter on.
func enableSnippet(key string) string {
	return fmt.Sprintf(`{
  "identifiers": {
    %s: {
      "enabled": true
    }
  }
}`, quote(key))
}

// thresholdSnippet is the policy fragment that sets a confidence cutoff for one
// entity on a filter that takes them.
func thresholdSnippet(key string, entity string, value float64) string {
	return fmt.Sprintf(`{
  "identifiers": {
    %s: {
      "thresholds": {
        %s: %s
      }
    }
  }
}`, quote(key), quote(entity), formatFloat(value))
}

func quote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// formatFloat writes a threshold the way a policy author would, without a long
// binary-fraction tail.
func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", f), "0"), ".")
}
