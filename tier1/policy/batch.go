package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ApproveBatch is the Phase 3 envelope hook; every record still passes the
// existing Phase 2 schema. An empty records array is an idle heartbeat.
func (g *Gate) ApproveBatch(input []byte) (Approved, error) {
	obj, reason := decode(input)
	stripped := 0
	clean := map[string]any{}
	if reason == "" {
		allowed := map[string]bool{"tenant": true, "collector_id": true, "region": true, "seq": true, "batch_id": true, "sent_at": true, "records": true}
		for key, value := range obj {
			if !allowed[key] {
				stripped++
				continue
			}
			if key == "records" {
				continue
			}
			if !validValue(value, g.cfg.Fields[key]) || !validIdentityField(key, value) {
				reason = "batch_field"
				break
			}
			clean[key] = value
		}
		for key := range allowed {
			if _, ok := obj[key]; !ok {
				reason = "batch_required"
			}
		}
		if reason == "" {
			records, ok := obj["records"].([]any)
			if !ok || len(records) > 256 {
				reason = "batch_records"
			} else {
				out := make([]any, 0, len(records))
				for _, value := range records {
					record, ok := value.(map[string]any)
					if !ok {
						reason = "batch_records"
						break
					}
					result, why := g.sanitize(record, &stripped)
					if why != "" {
						reason = why
						break
					}
					for _, key := range []string{"tenant", "collector_id", "region"} {
						if result[key] != clean[key] {
							reason = "batch_identity"
						}
					}
					if reason != "" {
						break
					}
					out = append(out, result)
				}
				clean["records"] = out
			}
		}
	}
	var approved Approved
	if reason == "" {
		data, err := json.Marshal(clean)
		if err != nil {
			reason = "encoding"
		} else {
			sum := sha256.Sum256(data)
			approved = Approved{data: data, hash: hex.EncodeToString(sum[:]), stripped: stripped}
		}
	}
	g.mu.Lock()
	g.counts.Stripped += uint64(stripped)
	if reason == "" {
		g.counts.Approved++
	} else {
		g.counts.Blocked++
	}
	g.mu.Unlock()
	if g.report != nil {
		g.reportMu.Lock()
		action := "approved"
		if reason != "" {
			action = "blocked"
		}
		g.report(Event{Action: action, Reason: reason, Stripped: stripped, SHA256: approved.hash})
		g.reportMu.Unlock()
	}
	if reason != "" {
		return Approved{}, fmt.Errorf("policy violation: %s", reason)
	}
	return approved, nil
}
