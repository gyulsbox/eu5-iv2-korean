package validate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// KeyDigestHeader marks a file produced by `iv2loc keys`.
const KeyDigestHeader = "# iv2loc key digest v1"

// KeyRef is a localization key together with the layer it was defined in.
type KeyRef struct {
	Layer string
	Key   string
}

// ReadKeyDigest parses a digest written by `iv2loc keys`. The format is one
// `layer<TAB>key` per line, with `#` comments; a line carrying no tab is read
// as a bare key so that a hand-written list also works.
func ReadKeyDigest(r io.Reader) ([]KeyRef, error) {
	var out []KeyRef
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		layer, key, found := strings.Cut(line, "\t")
		if !found {
			layer, key = "", strings.TrimSpace(line)
		}
		if key == "" {
			continue
		}
		out = append(out, KeyRef{Layer: layer, Key: key})
	}
	return out, sc.Err()
}

// ReadKeyDigestFile is ReadKeyDigest over a path.
func ReadKeyDigestFile(path string) ([]KeyRef, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	refs, err := ReadKeyDigest(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return refs, nil
}
