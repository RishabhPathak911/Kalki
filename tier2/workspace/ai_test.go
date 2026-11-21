package workspace

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAITenantBoundaryAndAudit(t *testing.T) {
	t.Setenv("KALKI_AI_URL", "")
	f := phase5(t)
	alpha := login(t, f.aps, "tenant_alpha")
	beta := login(t, f.eus, "tenant_beta")
	id := activeIncident(t, f, alpha)
	body, _ := json.Marshal(map[string]string{"incident_id": id})
	if code, _ := request(t, f.eus, "POST", "/api/ai/analyze-incident", beta, body, ""); code != 404 {
		t.Fatal(code)
	}
	if code, _ := request(t, f.aps, "POST", "/api/ai/analyze-incident", alpha, body, ""); code != 200 {
		t.Fatal(code)
	}
	secret := "password=FakeSecretXYZ 192.168.1.2"
	b, _ := json.Marshal(map[string]string{"question": secret})
	code, out := request(t, f.aps, "POST", "/api/ai/ask", alpha, b, "")
	if code != 200 || strings.Contains(string(out), "FakeSecretXYZ") {
		t.Fatal("AI leaked question")
	}
	_, audit := request(t, f.aps, "GET", "/api/audit", alpha, nil, "")
	if strings.Contains(string(audit), secret) || !strings.Contains(string(audit), "ai_query") {
		t.Fatal("audit prompt leak or missing event")
	}
	if code, _ := request(t, f.aps, "GET", "/api/validation", "", nil, ""); code != 401 {
		t.Fatal("unauthenticated report")
	}
}
