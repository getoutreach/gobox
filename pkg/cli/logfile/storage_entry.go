// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the entry codec for the log file.

package logfile

import (
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

// EntryType is the type of entry in the log file
type EntryType int

const (
	// EntryTypeMetadata is a metadata entry which is equal to
	// a Metadata struct
	EntryTypeMetadata EntryType = iota

	// EntryTypeFrame is a frame entry which is equal to a Frame struct
	EntryTypeFrame
)

// Entry is an entry in the log file
type Entry struct {
	f *Frame
	m *Metadata
}

// NewEntryFromFrame creates an entry from a frame
func NewEntryFromFrame(f *Frame) Entry {
	return Entry{
		f: f,
	}
}

// NewEntryFromMetadata creates an entry from metadata
func NewEntryFromMetadata(m *Metadata) Entry {
	return Entry{
		m: m,
	}
}

// NewFrame creates a new frame entry
func NewFrameEntry(delay time.Duration, bytes []byte) Entry {
	return NewEntryFromFrame(&Frame{
		EntryMetadata: EntryMetadata{
			Type: EntryTypeFrame,
		},
		Delay: delay,
		Bytes: bytes,
	})
}

// NewMetadata creates a new metadata entry
func NewMetadataEntry(startedAt time.Time, command string, args []string) Entry {
	return NewEntryFromMetadata(&Metadata{
		EntryMetadata: EntryMetadata{
			Type: EntryTypeMetadata,
		},
		StartedAt: startedAt,
		Command:   command,
		Args:      args,
	})
}

// MarshalJSON implements json.Marshaler for an entry
func (e Entry) MarshalJSON() ([]byte, error) {
	if e.IsFrame() {
		return jsoniter.Marshal(e.AsFrame())
	}

	if e.IsMetadata() {
		return jsoniter.Marshal(e.AsMetadata())
	}

	return nil, fmt.Errorf("unknown entry type: %v", e)
}

// UnmarshalJSON implements json.Unmarshaler picking the correct
// type of entry based on the type field
func (e *Entry) UnmarshalJSON(data []byte) error {
	var em EntryMetadata
	if err := jsoniter.Unmarshal(data, &em); err != nil {
		return errors.Wrap(err, "unmarshaling entry metadata")
	}

	switch em.Type {
	case EntryTypeMetadata:
		e.m = &Metadata{}
		if err := jsoniter.Unmarshal(data, e.m); err != nil {
			return errors.Wrap(err, "unmarshaling metadata")
		}
	case EntryTypeFrame:
		e.f = &Frame{}
		if err := jsoniter.Unmarshal(data, e.f); err != nil {
			return errors.Wrap(err, "unmarshaling frame")
		}
	default:
		return fmt.Errorf("unknown entry type %v: '%s'", em.Type, string(data))
	}

	return nil
}

// IsFrame returns true if the entry is a frame
func (e Entry) IsFrame() bool {
	return e.f != nil
}

// IsMetadata returns true if the entry is metadata
func (e Entry) IsMetadata() bool {
	return e.m != nil
}

// AsMetadata returns the metadata from the current entry, or nil
// if it's not metadata
func (e Entry) AsMetadata() *Metadata {
	return e.m
}

// AsFrame returns the current frame or nil if it's not a frame
func (e Entry) AsFrame() *Frame {
	return e.f
}

// EntryMetadata is the basic metadata for an entry that must
// be present in all entries
type EntryMetadata struct {
	// Type is the type of entry in the log file
	Type EntryType `json:"t"`
}
