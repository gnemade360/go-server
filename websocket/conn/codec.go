package conn

import (
	"encoding/json"
	"fmt"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

// JSONCodec encodes and decodes application messages as JSON text
// frames with a configurable nesting-depth cap.
type JSONCodec struct {
	MaxDepth int
}

func (c JSONCodec) Encode(v interface{}) (types.Message, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return types.Message{}, err
	}
	return types.Message{Type: types.TextMessage, Data: b}, nil
}

func (c JSONCodec) Decode(m types.Message, dst interface{}) error {
	if c.MaxDepth > 0 {
		if err := checkJSONDepth(m.Data, c.MaxDepth); err != nil {
			return err
		}
	}
	return json.Unmarshal(m.Data, dst)
}

func checkJSONDepth(data []byte, max int) error {
	depth := 0
	inString, escaped := false, false
	for _, b := range data {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > max {
				return fmt.Errorf("websocket: JSON nesting exceeds %d levels", max)
			}
		case '}', ']':
			depth--
		}
	}
	return nil
}
