package cacheinstance

import (
	"sort"
	"strings"
)

func (r *Registry) Lookup(kind, id string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.closed {
		if k := r.kinds[strings.TrimSpace(kind)]; k != nil {
			if e := k.entries[strings.TrimSpace(id)]; e != nil && e.value != nil {
				return e.value, true
			}
		}
	}
	return nil, false
}

func (r *Registry) collectInstances(kind string, all bool) ([]InstanceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []InstanceInfo{}
	if r.closed {
		return out, ErrRegistryClosed
	}
	if !all {
		var err error
		kind, err = clean(kind)
		if err != nil {
			return nil, err
		}
		if k := r.kinds[kind]; k == nil || (k.factory == nil && !hasInstances(k)) {
			return nil, ErrKindNotFound
		}
	}
	for name, k := range r.kinds {
		if !all && name != kind {
			continue
		}
		for id, e := range k.entries {
			if e.value != nil {
				out = append(out, InstanceInfo{name, id, e.source})
			}
		}
	}
	return out, nil
}

func (r *Registry) snapshot(kind string, all bool) ([]InstanceInfo, error) {
	out, err := r.collectInstances(kind, all)
	if err != nil {
		return out, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

func (r *Registry) Instances(kind string) ([]InstanceInfo, error) { return r.snapshot(kind, false) }

func (r *Registry) AllInstances() []InstanceInfo { out, _ := r.snapshot("", true); return out }
