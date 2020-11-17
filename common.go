package serviced

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func toJSON(v interface{}) string {
	jsonBytes, err := json.Marshal(v)
	if err == nil {
		return string(jsonBytes)
	}
	return err.Error()
}

func envReplaceEmpty(values map[string]interface{}, val string, empty bool) string {
	reg := regexp.MustCompile("\\$\\{[^\\}]*\\}")
	var rval string
	val = reg.ReplaceAllStringFunc(val, func(m string) string {
		keys := strings.Split(strings.Trim(m, "${}\t "), ",")
		for _, key := range keys {
			if v, ok := values[key]; ok {
				rval = fmt.Sprintf("%v", v)
			} else {
				rval = os.Getenv(key)
			}
			if len(rval) > 0 {
				break
			}
		}
		if len(rval) > 0 {
			return rval
		}
		if empty {
			return ""
		}
		return m
	})
	return val
}
