package main

import (
	"io"
	"strings"
	"testing"
)

func TestRiskCoverage(t *testing.T) {
	policy := `[{"file":"pkg/provider/provider.go","function":"deleteEndpoint","minimum":90,"risk":"deletion"}]`
	row := module + "pkg/provider/provider.go:42: deleteEndpoint "
	for _, tc := range []struct {
		name, policy, report string
		wantError            bool
	}{
		{"passing", policy, row + "90.0%\ntotal: (statements) 1.0%\n", false},
		{"high-average-cannot-hide-risk", policy, row + "89.9%\ntotal: (statements) 99.0%\n", true},
		{"missing-function", policy, "total: (statements) 100.0%\n", true},
		{"empty-report", policy, "", true},
		{"missing-total", policy, row + "100.0%\n", true},
		{"empty-policy", "[]", row + "100.0%\n", true},
		{"unknown-key", `[{"minimun":90}]`, "", true},
		{"invalid-policy", "{", "", true},
		{"trailing-json", policy + "{}", "", true},
		{"invalid-minimum", strings.ReplaceAll(policy, "90", "101"), row + "100.0%\ntotal: (statements) 99.0%\n", true},
		{"nan", policy, row + "NaN%\n", true},
		{"out-of-range", policy, row + "101%\n", true},
		{"malformed", policy, "garbage\n", true},
		{"bad-location", policy, "provider.go deleteEndpoint 100.0%\n", true},
		{"bad-line", policy, module + "pkg/provider/provider.go:x: deleteEndpoint 100.0%\n", true},
		{"wrong-module", policy, "other/provider.go:1: deleteEndpoint 100.0%\n", true},
		{"duplicate-rule", strings.TrimSuffix(policy, "]") + "," + strings.TrimPrefix(policy, "["), row + "100.0%\ntotal: (statements) 99.0%\n", true},
		{"duplicate-method-is-conservative", policy, row + "50.0%\n" + row + "100.0%\ntotal: (statements) 99.0%\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := check(strings.NewReader(tc.policy), strings.NewReader(tc.report), io.Discard)
			if (err != nil) != tc.wantError {
				t.Fatalf("got %v; want error=%v", err, tc.wantError)
			}
		})
	}
}
