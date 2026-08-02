package mcp

import "encoding/json"

// UnmarshalJSON accepts the lifecycle fields used by legacy clients while only
// retaining the protocol version needed for compatibility negotiation.
func (p *initializeParams) UnmarshalJSON(data []byte) error {
	var envelope struct {
		ProtocolVersion string         `json:"protocolVersion"`
		Capabilities    map[string]any `json:"capabilities"`
		ClientInfo      map[string]any `json:"clientInfo"`
		Meta            map[string]any `json:"_meta"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	p.ProtocolVersion = envelope.ProtocolVersion
	return nil
}

// UnmarshalJSON deliberately accepts latest per-request metadata and optional
// multi-round-trip fields. The mss tools are stateless, so those fields do not
// need to be retained by this server.
func (p *callToolParams) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Name           string         `json:"name"`
		Arguments      map[string]any `json:"arguments"`
		Meta           map[string]any `json:"_meta"`
		InputResponses map[string]any `json:"inputResponses"`
		RequestState   string         `json:"requestState"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	p.Name = envelope.Name
	p.Arguments = envelope.Arguments
	return nil
}
