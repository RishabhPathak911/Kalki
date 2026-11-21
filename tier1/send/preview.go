// Package send accepts only policy-approved payloads. Phase 2 provides a local
// NDJSON preview, not HTTP delivery, acknowledgement, or durability.
package send

import (
	"fmt"
	"io"

	"kalki/tier1/policy"
)

func Preview(w io.Writer, payload policy.Approved) error {
	data := payload.Bytes()
	if len(data) == 0 || payload.SHA256() == "" {
		return fmt.Errorf("send: unapproved payload")
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}
