package workspace

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const installer = `#!/bin/sh
set -eu
umask 077
base=''
ca=''
tenant=''
region=''
collector=''
token=''
window='5s'
probe=''
while [ "$#" -gt 0 ]; do
 case "$1" in
  --base-url) base="$2"; shift 2 ;;
  --ca-cert) ca="$2"; shift 2 ;;
  --ca-cert=*) ca="${1#*=}"; shift ;;
  --tenant=*) tenant="${1#*=}"; shift ;;
  --region=*) region="${1#*=}"; shift ;;
  --collector-id=*) collector="${1#*=}"; shift ;;
  --token=*) token="${1#*=}"; shift ;;
  --window=*) window="${1#*=}"; shift ;;
  --probe-target=*) probe="${1#*=}"; shift ;;
  --tier2-url=*|--key-file=*|--wal-path=*) shift ;;
  *) echo 'Unsupported installer flag' >&2; exit 1 ;;
 esac
done
case "$base" in https://*) ;; *) echo 'HTTPS base URL required' >&2; exit 1 ;; esac
case "$tenant" in ''|*[!a-zA-Z0-9_-]*) echo 'Invalid tenant' >&2; exit 1 ;; esac
case "$collector" in host_????????) ;; *) echo 'Invalid collector ID' >&2; exit 1 ;; esac
case "$collector" in *[!a-zA-Z0-9_-]*) exit 1 ;; esac
[ -n "$token" ] || { echo 'Collector token required' >&2; exit 1; }
[ -z "$ca" ] || [ -f "$ca" ] || { echo 'Private CA file missing' >&2; exit 1; }
case "$(uname -s)/$(uname -m)" in
 Linux/x86_64) binary=collector-linux-amd64 ;;
 Linux/aarch64|Linux/arm64) binary=collector-linux-arm64 ;;
 Darwin/arm64) binary=collector-darwin-arm64 ;;
 Darwin/x86_64) binary=collector-darwin-amd64 ;;
 *) echo 'Unsupported platform; use install.ps1 for Windows' >&2; exit 1 ;;
esac
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
download() {
 if [ -n "$ca" ]; then curl --fail --silent --show-error --proto '=https' --cacert "$ca" "$1" -o "$2"
 else curl --fail --silent --show-error --proto '=https' "$1" -o "$2"; fi
}
download "$base/downloads/$binary" "$work/collector"
download "$base/downloads/allowlist.yaml" "$work/allowlist.yaml"
chmod 700 "$work/collector"
state="$HOME/.kalki"
mkdir -p "$state"
chmod 700 "$state"
export KALKI_TOKEN="$token"
if [ -n "$ca" ]; then export KALKI_CA_CERT="$ca"; else export KALKI_CA_CERT=''; fi
"$work/collector" --tenant="$tenant" --region="$region" --collector-id="$collector" --tier2-url="$base" --probe-target="$probe" --window="$window" --key-file="$state/$tenant.key" --wal-path="$state/$collector.wal" --proof-addr=127.0.0.1:0 --allowlist="$work/allowlist.yaml"
`

func (g *Gateway) download(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		failure(w, 405)
		return
	}
	if r.URL.Path == "/install.sh" {
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		_, _ = w.Write([]byte(installer))
		return
	}
	if r.URL.Path == "/install.ps1" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(windowsInstaller))
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/downloads/")
	switch name {
	case "collector-linux-amd64", "collector-linux-arm64", "collector-darwin-arm64", "collector-darwin-amd64", "collector-windows-amd64.exe", "allowlist.yaml":
	default:
		failure(w, 404)
		return
	}
	path := filepath.Join(g.releases, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		failure(w, 404)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		failure(w, 404)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, info.ModTime(), f)
}
