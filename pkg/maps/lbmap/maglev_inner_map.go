// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package lbmap

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/cilium/cilium/pkg/bpf"
	"github.com/cilium/cilium/pkg/ebpf"
	"github.com/cilium/cilium/pkg/loadbalancer"
)

const MaglevInnerMapName = "cilium_maglev_inner"

// MaglevBackendLen represents the length of a single backend ID
// in a Maglev lookup table.
var MaglevBackendLen = uint32(unsafe.Sizeof(loadbalancer.BackendID(0)))

// MaglevInnerMap represents a maglev inner map.
type MaglevInnerMap ebpf.Map

// TableSize returns the amount of backends this map can hold as a value.
func (m *MaglevInnerMap) TableSize() uint32 {
	return m.Map.ValueSize() / uint32(MaglevBackendLen)
}

// UpdateBackends updates the maglev inner map's list of backends.
func (m *MaglevInnerMap) UpdateBackends(backends []loadbalancer.BackendID) error {
	// Backends are stored at inner map key zero.
	var key MaglevInnerKey
	return m.Map.Update(key, backends, 0)
}

// MaglevInnerKey is the key of a maglev inner map.
type MaglevInnerKey struct {
	Zero uint32
}

// New and String implement bpf.MapKey
func (k *MaglevInnerKey) New() bpf.MapKey { return &MaglevInnerKey{} }
func (k *MaglevInnerKey) String() string  { return fmt.Sprintf("%d", k.Zero) }

// MaglevInnerVal is the value of a maglev inner map.
type MaglevInnerVal struct {
	BackendIDs []loadbalancer.BackendID
}

// newMaglevInnerMapSpec returns the spec for a maglev inner map.
func newMaglevInnerMapSpec(tableSize uint32) *ebpf.MapSpec {
	return &ebpf.MapSpec{
		Name:       MaglevInnerMapName,
		Type:       ebpf.Array,
		KeySize:    uint32(unsafe.Sizeof(MaglevInnerKey{})),
		ValueSize:  MaglevBackendLen * tableSize,
		MaxEntries: 1,
	}
}

// createMaglevInnerMap creates a new Maglev inner map in the kernel
// using the given table size.
func createMaglevInnerMap(logger *slog.Logger, tableSize uint32) (*MaglevInnerMap, error) {
	spec := newMaglevInnerMapSpec(tableSize)

	m := ebpf.NewMap(logger, spec)
	if err := m.OpenOrCreate(); err != nil {
		return nil, err
	}

	return (*MaglevInnerMap)(m), nil
}

// MaglevInnerMapFromID returns a new object representing the maglev inner map
// identified by an ID.
func MaglevInnerMapFromID(logger *slog.Logger, id uint32) (*MaglevInnerMap, error) {
	m, err := ebpf.MapFromID(logger, int(id))
	if err != nil {
		return nil, err
	}

	return (*MaglevInnerMap)(m), nil
}

// Lookup returns the value associated with a given key for a maglev inner map.
func (m *MaglevInnerMap) Lookup(key *MaglevInnerKey) (*MaglevInnerVal, error) {
	value := &MaglevInnerVal{
		BackendIDs: make([]loadbalancer.BackendID, m.TableSize()),
	}

	if err := m.Map.Lookup(key, &value.BackendIDs); err != nil {
		return nil, err
	}

	return value, nil
}

// DumpBackends returns the first key of the map as stringified ints for dumping purposes.
func (m *MaglevInnerMap) DumpBackends() (string, error) {
	// A service's backend array sits at the first key of the inner map.
	var key MaglevInnerKey
	val, err := m.Lookup(&key)
	if err != nil {
		return "", fmt.Errorf("lookup up first inner map key (backends): %w", err)
	}

	return fmt.Sprintf("%v", val.BackendIDs), nil
}
