package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type ID string

func (id *ID) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("decode id: %w", err)
		}
	} else {
		if !validJSONNumber(string(data)) {
			return fmt.Errorf("decode id: expected string or number")
		}
		text = string(data)
	}
	*id = ID(text)
	return nil
}

func (id ID) MarshalJSON() ([]byte, error) { return json.Marshal(string(id)) }

type Int int64

func (value *Int) UnmarshalJSON(data []byte) error {
	text, err := scalarText(data)
	if err != nil {
		return fmt.Errorf("decode integer: %w", err)
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("decode integer %q: %w", text, err)
	}
	*value = Int(parsed)
	return nil
}

type Bool bool

func (value *Bool) UnmarshalJSON(data []byte) error {
	text, err := scalarText(data)
	if err != nil {
		return fmt.Errorf("decode boolean: %w", err)
	}
	switch strings.ToLower(text) {
	case "true", "1", "yes":
		*value = true
	case "false", "0", "no":
		*value = false
	default:
		return fmt.Errorf("decode boolean %q: expected true/false, 1/0, or yes/no", text)
	}
	return nil
}

type Money string

func (value *Money) UnmarshalJSON(data []byte) error {
	text, err := scalarText(data)
	if err != nil || !validJSONNumber(text) {
		return fmt.Errorf("decode decimal %q: expected a finite JSON number", text)
	}
	*value = Money(text)
	return nil
}

func (value Money) MarshalJSON() ([]byte, error) { return json.Marshal(string(value)) }

type StringOrSlice []string

func (value *StringOrSlice) UnmarshalJSON(data []byte) error {
	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		*value = many
		return nil
	}
	var one string
	if err := json.Unmarshal(data, &one); err != nil {
		return fmt.Errorf("decode string or string list: %w", err)
	}
	*value = []string{one}
	return nil
}

type Ref struct {
	ID   ID     `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Refs []Ref

func (refs *Refs) UnmarshalJSON(data []byte) error {
	var many []Ref
	if err := json.Unmarshal(data, &many); err == nil {
		*refs = many
		return nil
	}
	var one Ref
	if err := json.Unmarshal(data, &one); err != nil {
		return fmt.Errorf("decode reference list: %w", err)
	}
	*refs = []Ref{one}
	return nil
}

func scalarText(data []byte) (string, error) {
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return "", err
		}
		return text, nil
	}
	return string(data), nil
}

func validJSONNumber(text string) bool {
	if text == "" || strings.ContainsAny(text, "NaInfxX") {
		return false
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return false
	}
	var extra any
	if decoder.Decode(&extra) == nil {
		return false
	}
	parsed, err := strconv.ParseFloat(number.String(), 64)
	return err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
}
