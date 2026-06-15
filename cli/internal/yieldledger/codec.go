package yieldledger

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// wireEvent is the on-disk shape of an Event: the common envelope plus a single
// `body` object whose contents are the typed payload for `event`. The Go Event
// keeps the three bodies as discriminated pointers; this wire form gives the
// schema-checkable {event, bead_id, run_id, ts, body} envelope.
type wireEvent struct {
	Event  string          `json:"event"`
	BeadID string          `json:"bead_id"`
	RunID  string          `json:"run_id"`
	TS     string          `json:"ts"`
	Body   json.RawMessage `json:"body"`
}

// MarshalJSON renders an Event as the wire envelope with its typed body nested
// under `body`, discriminated by `event`.
func (e Event) MarshalJSON() ([]byte, error) {
	var body any
	switch e.Event {
	case EventAccept:
		if e.Accept == nil {
			return nil, fmt.Errorf("yieldledger: accept event missing body")
		}
		body = e.Accept
	case EventGateVerdict:
		if e.GateVerdict == nil {
			return nil, fmt.Errorf("yieldledger: gate-verdict event missing body")
		}
		body = e.GateVerdict
	case EventUsage:
		if e.Usage == nil {
			return nil, fmt.Errorf("yieldledger: usage event missing body")
		}
		body = e.Usage
	default:
		return nil, fmt.Errorf("yieldledger: unknown event %q", e.Event)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return json.Marshal(wireEvent{
		Event:  e.Event,
		BeadID: e.BeadID,
		RunID:  e.RunID,
		TS:     e.TS,
		Body:   raw,
	})
}

// UnmarshalJSON parses the wire envelope and decodes the nested `body` into the
// typed pointer selected by `event`. The schema is closed
// (additionalProperties:false): an unknown/misspelled field on the envelope OR
// the typed body is a REJECT, never a silent drop.
func (e *Event) UnmarshalJSON(data []byte) error {
	var w wireEvent
	if err := strictDecode(data, &w); err != nil {
		return fmt.Errorf("yieldledger: decode event envelope: %w", err)
	}
	e.Event = w.Event
	e.BeadID = w.BeadID
	e.RunID = w.RunID
	e.TS = w.TS
	e.Accept = nil
	e.GateVerdict = nil
	e.Usage = nil

	switch w.Event {
	case EventAccept:
		var b AcceptBody
		if err := unmarshalBody(w.Body, &b); err != nil {
			return err
		}
		e.Accept = &b
	case EventGateVerdict:
		var b GateVerdictBody
		if err := unmarshalBody(w.Body, &b); err != nil {
			return err
		}
		e.GateVerdict = &b
	case EventUsage:
		var b UsageBody
		if err := unmarshalBody(w.Body, &b); err != nil {
			return err
		}
		e.Usage = &b
	default:
		return fmt.Errorf("yieldledger: unknown event %q", w.Event)
	}
	return nil
}

// unmarshalBody decodes a non-empty raw body into dst, failing closed on an
// absent body so a typed event can never load with a nil payload. The decode is
// STRICT: an unknown body field (closed schema, additionalProperties:false) is
// a reject, not a silent drop.
func unmarshalBody(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return fmt.Errorf("yieldledger: event missing body")
	}
	if err := strictDecode(raw, dst); err != nil {
		return fmt.Errorf("yieldledger: decode event body: %w", err)
	}
	return nil
}

// strictDecode unmarshals data into dst rejecting any field not present in the
// destination type (closed-schema enforcement for the envelope and every typed
// body).
func strictDecode(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}
