package presentation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
)

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic any
	if err = decoder.Decode(&generic); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err = writeCanonicalJSON(&output, generic); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func canonicalJSONBytes(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic any
	if err := decoder.Decode(&generic); err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := writeCanonicalJSON(&output, generic); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writeCanonicalJSON(output *bytes.Buffer, value any) error {
	switch current := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		output.WriteString(strconv.FormatBool(current))
	case string:
		encoded, _ := json.Marshal(current)
		output.Write(encoded)
	case json.Number:
		if _, err := strconv.ParseFloat(current.String(), 64); err != nil {
			return fmt.Errorf("invalid JSON number %q", current)
		}
		output.WriteString(current.String())
	case []any:
		output.WriteByte('[')
		for index := range current {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := writeCanonicalJSON(output, current[index]); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			encoded, _ := json.Marshal(key)
			output.Write(encoded)
			output.WriteByte(':')
			if err := writeCanonicalJSON(output, current[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}

func sha256Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkJSONValue(decoder, "$", 0); err != nil {
		return err
	}
	return ensureJSONTokenEOF(decoder)
}

func walkJSONValue(decoder *json.Decoder, path string, depth int) error {
	if depth > 32 {
		return fmt.Errorf("%s exceeds maximum JSON depth", path)
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return keyErr
			}
			key, keyOK := keyToken.(string)
			if !keyOK {
				return fmt.Errorf("%s contains a non-string object key", path)
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("%s.%s is duplicated", path, key)
			}
			seen[key] = struct{}{}
			if err = walkJSONValue(decoder, path+"."+key, depth+1); err != nil {
				return err
			}
		}
		end, endErr := decoder.Token()
		if endErr != nil {
			return endErr
		}
		if end != json.Delim('}') {
			return fmt.Errorf("%s object is not closed", path)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err = walkJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index), depth+1); err != nil {
				return err
			}
			index++
		}
		end, endErr := decoder.Token()
		if endErr != nil {
			return endErr
		}
		if end != json.Delim(']') {
			return fmt.Errorf("%s array is not closed", path)
		}
	default:
		return fmt.Errorf("%s contains unexpected delimiter %q", path, delimiter)
	}
	return nil
}

func ensureJSONTokenEOF(decoder *json.Decoder) error {
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}
