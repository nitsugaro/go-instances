package cacheinstance

import (
	"context"
	"sync"
)

type gate struct {
	mu               sync.Mutex
	readers, waiting int
	writer           bool
	changed          chan struct{}
}

func (g *gate) signal() {
	if g.changed != nil {
		close(g.changed)
	}
	g.changed = make(chan struct{})
}

func (g *gate) lock(ctx context.Context, write bool) error {
	g.mu.Lock()
	if g.changed == nil {
		g.changed = make(chan struct{})
	}
	if write {
		g.waiting++
	}
	for {
		if err := ctx.Err(); err != nil {
			if write {
				g.waiting--
				g.signal()
			}
			g.mu.Unlock()
			return err
		}
		if !g.writer && ((write && g.readers == 0) || (!write && g.waiting == 0)) {
			if write {
				g.waiting--
				g.writer = true
			} else {
				g.readers++
			}
			g.mu.Unlock()
			return nil
		}
		ch := g.changed
		g.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
		}
		g.mu.Lock()
	}
}

func (g *gate) unlock(write bool) {
	g.mu.Lock()
	if write {
		g.writer = false
	} else {
		g.readers--
	}
	g.signal()
	g.mu.Unlock()
}
