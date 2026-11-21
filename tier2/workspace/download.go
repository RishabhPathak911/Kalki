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
while [ "$#" -gt 0 ]; do
 case "$1" in
  --base-url) base="$2"; shift 2 ;;
  --ca-cert) ca="$2"; shift 2 ;;
  *) break ;;
 esac
done
case "$base" in https://*) ;; *) echo 'HTTPS base URL required' >&2; exit 1 ;; esac
[ -f "$ca" ] || { echo 'Copy the trusted CA to ca.pem first' >&2; exit 1; }
case "$(uname -s)/$(uname -m)" in
 Linux/x86_64) binary=collector-linux-amd64 ;;
 Darwin/arm64) binary=collector-darwin-arm64 ;;
 *) echo 'Unsupported platform; use the Windows binary directly on Windows' >&2; exit 1 ;;
esac
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
curl --fail --silent --show-error --proto '=https' --cacert "$ca" "$base/downloads/$binary" -o "$work/collector"
curl --fail --silent --show-error --proto '=https' --cacert "$ca" "$base/downloads/allowlist.yaml" -o "$work/allowlist.yaml"
chmod 700 "$work/collector"
"$work/collector" "$@" --allowlist="$work/allowlist.yaml"
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
	name := strings.TrimPrefix(r.URL.Path, "/downloads/")
	switch name {
	case "collector-linux-amd64", "collector-darwin-arm64", "collector-windows-amd64.exe", "allowlist.yaml":
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
