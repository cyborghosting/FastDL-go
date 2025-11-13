package utils

import (
	"log"

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
