package api

import "encoding/json"

type pageEnvelope struct {
	Data   []json.RawMessage `json:"data"`
	Paging struct {
		Cursors struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"cursors"`
		Next string `json:"next"`
	} `json:"paging"`
}

func decodePage(body []byte) ([]any, string, error) {
	var envelope pageEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		var direct []any
		if directErr := json.Unmarshal(body, &direct); directErr != nil {
			return nil, "", err
		}
		return direct, "", nil
	}
	items := make([]any, 0, len(envelope.Data))
	for _, raw := range envelope.Data {
		var item any
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, "", err
		}
		items = append(items, item)
	}
	return items, envelope.Paging.Cursors.After, nil
}
