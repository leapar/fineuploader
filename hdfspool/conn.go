package pool

import (
	"sync"

	dfs "github.com/colinmarc/hdfs"
)

// PoolConn is a wrapper around net.Conn to modify the the behavior of
// net.Conn's Close() method.
type PoolConn struct {
	*dfs.Client
	mu       sync.RWMutex
	c        *clientPool
	unusable bool
}

// Close puts the given connects back to the pool instead of closing it.
func (p *PoolConn) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.unusable {
		if p.Client != nil {
			p.Client.Close()
			return nil
		}
		return nil
	}
	return p.c.put(p.Client)
}

// MarkUnusable marks the connection not usable any more, to let the pool close it instead of returning it to pool.
func (p *PoolConn) MarkUnusable() {
	p.mu.Lock()
	p.unusable = true
	p.mu.Unlock()
}

// newConn wraps a standard net.Conn to a poolConn net.Conn.
func (c *clientPool) wrapConn(conn *dfs.Client) *PoolConn {
	p := &PoolConn{c: c}
	p.Client = conn
	return p
}
