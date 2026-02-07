package utils

import (
	"log"
	"unicode/utf8"

	keyvalues "github.com/galaco/KeyValues"
	"github.com/gin-gonic/gin"
)

type KeyValueTraverser struct {
	kv *keyvalues.KeyValue
}

func NewKeyValueTraverser(kv *keyvalues.KeyValue) *KeyValueTraverser {
	return &KeyValueTraverser{kv: kv}
}

func (kvt *KeyValueTraverser) Traverse(path ...string) (*keyvalues.KeyValue, error) {
	return traverse(kvt.kv, path)
}

func TraverseKeyValue(kv *keyvalues.KeyValue, path ...string) (*keyvalues.KeyValue, error) {
	return traverse(kv, path)
}

func traverse(kv *keyvalues.KeyValue, path []string) (*keyvalues.KeyValue, error) {
	if len(path) == 0 {
		return kv, nil
	}

	find, err := kv.Find(path[0])
	if err != nil {
		return nil, err
	}

	return traverse(find, path[1:])
}

func DEBUG(format string, a ...any) {
	if gin.IsDebugging() {
		log.Printf(format, a...)
	}
}

// Regexp quoting utilities below adapted from regexp/regexp.go in the Go standard library.

var specialBytes [16]byte

func special(b byte) bool {
	return b < utf8.RuneSelf && specialBytes[b%16]&(1<<(b/16)) != 0
}

func init() {
	for _, b := range []byte(`-\.+*?()|[]{}^$`) {
		specialBytes[b%16] |= 1 << (b / 16)
	}
}

func RegexpQuote(s string) string {
	// A byte loop is correct because all metacharacters are ASCII.
	var i int
	for i = 0; i < len(s); i++ {
		if special(s[i]) {
			break
		}
	}
	// No meta characters found, so return original string.
	if i >= len(s) {
		return s
	}

	b := make([]byte, 2*len(s)-i)
	copy(b, s[:i])
	j := i
	for ; i < len(s); i++ {
		if special(s[i]) {
			b[j] = '\\'
			j++
		}
		b[j] = s[i]
		j++
	}
	return string(b[:j])
}

func RegexpSpecialRune(r rune) bool {
	return r < utf8.RuneSelf && special(byte(r))
}

func RegexpQuoteByte(b byte) string {
	if special(b) {
		return `\` + string(b)
	} else {
		return string(b)
	}
}

func RegexpQuoteRune(r rune) string {
	if r < utf8.RuneSelf {
		return RegexpQuoteByte(byte(r))
	}
	return string(r)
}
