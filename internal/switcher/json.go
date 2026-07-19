package switcher

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tidwall/sjson"
)

func setJSONString(data []byte, path, value string) ([]byte, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON")
	}
	hadFinalNewline := bytes.HasSuffix(data, []byte("\n"))
	updated, err := sjson.SetBytes(data, path, value)
	if err != nil {
		return nil, err
	}
	if hadFinalNewline && !bytes.HasSuffix(updated, []byte("\n")) {
		updated = append(updated, '\n')
	}
	return updated, nil
}

func readJSONString(data []byte, path ...string) (string, error) {
	if !json.Valid(data) {
		return "", fmt.Errorf("invalid JSON")
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", err
	}
	var current any = root
	for _, part := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return "", nil
		}
		current, ok = object[part]
		if !ok {
			return "", nil
		}
	}
	value, _ := current.(string)
	return value, nil
}
