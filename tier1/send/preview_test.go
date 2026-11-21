package send

import (
	"bytes"
	"testing"

	"kalki/tier1/policy"
)

func TestPreviewRejectsUnapprovedPayload(t *testing.T) {
	var out bytes.Buffer
	if err := Preview(&out, policy.Approved{}); err == nil || out.Len() != 0 {
		t.Fatal("unapproved data was written")
	}
}
