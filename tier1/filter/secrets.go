// Package filter is the only privacy stage that accepts raw samples. It emits
// numbers, fixed enums, timestamps and pseudonyms; never text, paths or tags.
package filter

import (
	"math"
	"regexp"
)

type rule struct {
	kind    string
	pattern *regexp.Regexp
}

var secretRules = []rule{
	{"private_key", regexp.MustCompile(`(?s)-----BEGIN (?:RSA |EC |DSA |OPENSSH |ENCRYPTED )?PRIVATE KEY-----.*?-----END (?:RSA |EC |DSA |OPENSSH |ENCRYPTED )?PRIVATE KEY-----`)},
	{"private_key", regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |ENCRYPTED )?PRIVATE KEY-----`)},
	{"bearer", regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+`)},
	{"aws_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"credential", regexp.MustCompile(`(?i)\b(?:api_key|token|secret|password|passwd)\s*[=:]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s]+)`)},
}
var entropyToken = regexp.MustCompile(`[A-Za-z0-9_+/=-]{20,}`)

// Scrub is useful for local inspection, but its text output is never exported.
// Counts can overlap when a credential assignment contains another secret type.
func Scrub(text string) (string, map[string]int) {
	counts := map[string]int{}
	for _, r := range secretRules {
		text = r.pattern.ReplaceAllStringFunc(text, func(string) string {
			counts[r.kind]++
			return "[REDACTED:" + r.kind + "]"
		})
	}
	text = entropyToken.ReplaceAllStringFunc(text, func(token string) string {
		if Entropy(token) < 4 {
			return token
		}
		counts["high_entropy"]++
		return "[REDACTED:high_entropy]"
	})
	return text, counts
}

func Entropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	counts := map[byte]int{}
	for i := range s {
		counts[s[i]]++
	}
	var entropy float64
	for _, n := range counts {
		p := float64(n) / float64(len(s))
		entropy -= p * math.Log2(p)
	}
	return entropy
}
