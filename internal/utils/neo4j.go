package utils

import (
	"fmt"
	"reflect"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
)

// NeoGetKeys returns a map of keys and values from a neo4j record
func NeoGetKeys(record *neo4j.Record, keys []string) (m map[string]interface{}, err error) {
	m = make(map[string]interface{})
	for _, k := range keys {
		v, ok := record.Get(k)
		if !ok {
			err = fmt.Errorf("no '%s' found", k)
			continue
		}
		switch reflect.TypeOf(v).String() {
		case "string":
			m[k] = v.(string)
		case "int64":
			m[k] = v.(int64)
		default:
			err = fmt.Errorf("unknown type '%s' for key '%s'", reflect.TypeOf(v).String(), k)
			panic(err)
		}
	}
	return
}
