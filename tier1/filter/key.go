package filter

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

type Identity struct{ key [32]byte }
type keyFile struct {
	Tenant string `json:"tenant"`
	Key    []byte `json:"key"`
}

// LoadIdentity binds a file to one tenant. A second tenant must use a different
// path, rather than accidentally sharing pseudonyms. Partial/corrupt files fail
// closed; existing keys are never silently regenerated.
func LoadIdentity(path, tenant string) (*Identity, error) {
	if tenant == "" {
		return nil, fmt.Errorf("tenant key: empty tenant")
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, fmt.Errorf("tenant key: cannot create directory")
		}
		var key [32]byte
		if _, err := rand.Read(key[:]); err != nil {
			return nil, fmt.Errorf("tenant key: random generation failed")
		}
		data, err := json.Marshal(keyFile{Tenant: tenant, Key: key[:]})
		if err != nil {
			return nil, fmt.Errorf("tenant key: encoding failed")
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, writeErr := f.Write(data)
			syncErr := f.Sync()
			closeErr := f.Close()
			if errors.Join(writeErr, syncErr, closeErr) != nil {
				return nil, fmt.Errorf("tenant key: write failed; inspect local key file")
			}
		} else if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("tenant key: cannot create file")
		}
	} else if err != nil {
		return nil, fmt.Errorf("tenant key: cannot inspect file")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("tenant key: expected regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("tenant key: permissions must be owner-only")
	}
	if info.Size() > 4096 {
		return nil, fmt.Errorf("tenant key: invalid file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tenant key: cannot read file")
	}
	data, readErr := io.ReadAll(io.LimitReader(f, 4097))
	closeErr := f.Close()
	if errors.Join(readErr, closeErr) != nil {
		return nil, fmt.Errorf("tenant key: read failed")
	}
	var stored keyFile
	if json.Unmarshal(data, &stored) != nil || stored.Tenant != tenant || len(stored.Key) != 32 {
		return nil, fmt.Errorf("tenant key: invalid file or tenant mismatch; use a separate key file per tenant")
	}
	id := &Identity{}
	copy(id.key[:], stored.Key)
	return id, nil
}

func (id *Identity) Pseudonym(kind, value string) string {
	mac := hmac.New(sha256.New, id.key[:])
	mac.Write([]byte(kind + "\x00" + value))
	return kind + "_" + hex.EncodeToString(mac.Sum(nil)[:4])
}
