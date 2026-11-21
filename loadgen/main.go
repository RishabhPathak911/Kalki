package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"kalki/internal/scale"
	"os"
	"time"
)

func main() {
	o := scale.Options{}
	flag.IntVar(&o.Collectors, "collectors", 500, "simulated collectors")
	flag.IntVar(&o.Tenants, "tenants", 20, "simulated tenants")
	flag.DurationVar(&o.Duration, "duration", 10*time.Second, "run duration")
	flag.StringVar(&o.URL, "url", "", "HTTPS receiver; empty uses in-process protocol harness")
	flag.StringVar(&o.CA, "ca", "data/certs/ca.pem", "trusted CA")
	flag.StringVar(&o.Credentials, "credentials", "", "local JSON of pre-minted collector identities; never printed")
	flag.StringVar(&o.Policy, "policy", "config/allowlist.yaml", "policy")
	flag.Parse()
	r, e := scale.Run(o)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	os.MkdirAll("data", 0700)
	if e = os.WriteFile("data/loadgen-report.json", b, 0600); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "SIMULATED: %d collectors / %d tenants, %d accepted, %.1f batches/s, p95 %.2f ms, peak %.1f MiB\n", r.Collectors, r.Tenants, r.Sent, r.Throughput, r.P95, r.Peak)
	if r.Failed > 0 {
		os.Exit(1)
	}
}
