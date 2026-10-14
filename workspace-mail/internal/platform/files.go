package platform

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func AtomicWrite(path string, data []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".workspace-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func SaveJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return AtomicWrite(path, b)
}
func LoadJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func SaveSecret(path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	b, e = Protect(b)
	if e != nil {
		return e
	}
	return AtomicWrite(path, b)
}
func LoadSecret(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	b, e = Unprotect(b)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func Inside(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, ":") || strings.Contains(rel, "\\") {
		return "", errors.New("invalid relative package path")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("package path escapes destination")
	}
	for _, part := range strings.Split(rel, "/") {
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if part == ".." || strings.TrimRight(part, " .") != part || strings.ContainsAny(part, "*?<>|\"\x00") || base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return "", errors.New("invalid Windows package path")
		}
	}
	if st, e := os.Lstat(root); e == nil && st.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("destination must not be a symbolic link")
	}
	path := filepath.Join(root, clean)
	current := root
	for _, part := range strings.Split(clean, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		if st, e := os.Lstat(current); e == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symbolic links are not accepted")
		}
	}
	return path, nil
}
func CopyFile(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	if e = os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
		return e
	}
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	ce := out.Close()
	if e == nil {
		e = ce
	}
	return e
}
func RequireNewDir(path string) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	if filepath.Dir(abs) == abs {
		return errors.New("choose a dedicated folder, not a drive root")
	}
	if entries, e := os.ReadDir(abs); e == nil && len(entries) > 0 {
		return fmt.Errorf("folder is not empty: %s", abs)
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	return os.MkdirAll(abs, 0700)
}
