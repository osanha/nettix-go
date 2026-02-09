package channel

import (
	"fmt"
	"sync"
)

// Pipeline is an ordered chain of handlers that process inbound and outbound events.
// Similar to Netty's ChannelPipeline, it supports named handlers for dynamic manipulation.
type Pipeline struct {
	mu       sync.RWMutex
	handlers []*namedHandler
}

// namedHandler wraps a handler with its name.
type namedHandler struct {
	name    string
	handler interface{}
}

// NewPipeline creates a new empty pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{
		handlers: make([]*namedHandler, 0),
	}
}

// AddLast adds a handler at the end of the pipeline.
func (p *Pipeline) AddLast(name string, handler interface{}) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.findIndex(name) >= 0 {
		panic(fmt.Sprintf("handler with name '%s' already exists", name))
	}

	p.handlers = append(p.handlers, &namedHandler{name: name, handler: handler})
	return p
}

// AddFirst adds a handler at the beginning of the pipeline.
func (p *Pipeline) AddFirst(name string, handler interface{}) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.findIndex(name) >= 0 {
		panic(fmt.Sprintf("handler with name '%s' already exists", name))
	}

	p.handlers = append([]*namedHandler{{name: name, handler: handler}}, p.handlers...)
	return p
}

// AddBefore adds a handler before the specified handler.
func (p *Pipeline) AddBefore(baseName, name string, handler interface{}) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.findIndex(name) >= 0 {
		panic(fmt.Sprintf("handler with name '%s' already exists", name))
	}

	idx := p.findIndex(baseName)
	if idx < 0 {
		panic(fmt.Sprintf("handler '%s' not found", baseName))
	}

	nh := &namedHandler{name: name, handler: handler}
	p.handlers = append(p.handlers[:idx], append([]*namedHandler{nh}, p.handlers[idx:]...)...)
	return p
}

// AddAfter adds a handler after the specified handler.
func (p *Pipeline) AddAfter(baseName, name string, handler interface{}) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.findIndex(name) >= 0 {
		panic(fmt.Sprintf("handler with name '%s' already exists", name))
	}

	idx := p.findIndex(baseName)
	if idx < 0 {
		panic(fmt.Sprintf("handler '%s' not found", baseName))
	}

	nh := &namedHandler{name: name, handler: handler}
	if idx == len(p.handlers)-1 {
		p.handlers = append(p.handlers, nh)
	} else {
		p.handlers = append(p.handlers[:idx+1], append([]*namedHandler{nh}, p.handlers[idx+1:]...)...)
	}
	return p
}

// Replace replaces an existing handler with a new one.
func (p *Pipeline) Replace(oldName, newName string, handler interface{}) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()

	idx := p.findIndex(oldName)
	if idx < 0 {
		panic(fmt.Sprintf("handler '%s' not found", oldName))
	}

	if oldName != newName && p.findIndex(newName) >= 0 {
		panic(fmt.Sprintf("handler with name '%s' already exists", newName))
	}

	p.handlers[idx] = &namedHandler{name: newName, handler: handler}
	return p
}

// Remove removes a handler from the pipeline.
func (p *Pipeline) Remove(name string) interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	idx := p.findIndex(name)
	if idx < 0 {
		return nil
	}

	removed := p.handlers[idx].handler
	p.handlers = append(p.handlers[:idx], p.handlers[idx+1:]...)
	return removed
}

// Get returns the handler with the specified name.
func (p *Pipeline) Get(name string) interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	idx := p.findIndex(name)
	if idx < 0 {
		return nil
	}
	return p.handlers[idx].handler
}

// Contains checks if a handler with the given name exists.
func (p *Pipeline) Contains(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.findIndex(name) >= 0
}

// Names returns all handler names in order.
func (p *Pipeline) Names() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, len(p.handlers))
	for i, h := range p.handlers {
		names[i] = h.name
	}
	return names
}

// Handlers returns all handlers in order.
func (p *Pipeline) Handlers() []interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	handlers := make([]interface{}, len(p.handlers))
	for i, h := range p.handlers {
		handlers[i] = h.handler
	}
	return handlers
}

// Size returns the number of handlers in the pipeline.
func (p *Pipeline) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.handlers)
}

// Clone creates a deep copy of the pipeline.
func (p *Pipeline) Clone() *Pipeline {
	p.mu.RLock()
	defer p.mu.RUnlock()

	clone := NewPipeline()
	for _, h := range p.handlers {
		clone.handlers = append(clone.handlers, &namedHandler{
			name:    h.name,
			handler: h.handler,
		})
	}
	return clone
}

// findIndex returns the index of a handler by name, or -1 if not found.
// Must be called with lock held.
func (p *Pipeline) findIndex(name string) int {
	for i, h := range p.handlers {
		if h.name == name {
			return i
		}
	}
	return -1
}

// Decoders returns all Decoder handlers in the pipeline.
func (p *Pipeline) Decoders() []Decoder {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var decoders []Decoder
	for _, h := range p.handlers {
		if d, ok := h.handler.(Decoder); ok {
			decoders = append(decoders, d)
		}
	}
	return decoders
}

// Encoders returns all Encoder handlers in the pipeline.
func (p *Pipeline) Encoders() []Encoder {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var encoders []Encoder
	for _, h := range p.handlers {
		if e, ok := h.handler.(Encoder); ok {
			encoders = append(encoders, e)
		}
	}
	return encoders
}

// InboundHandlers returns all InboundHandler handlers in the pipeline.
func (p *Pipeline) InboundHandlers() []InboundHandler {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var handlers []InboundHandler
	for _, h := range p.handlers {
		if ih, ok := h.handler.(InboundHandler); ok {
			handlers = append(handlers, ih)
		}
	}
	return handlers
}

// OutboundHandlers returns all OutboundHandler handlers in the pipeline.
func (p *Pipeline) OutboundHandlers() []OutboundHandler {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var handlers []OutboundHandler
	for _, h := range p.handlers {
		if oh, ok := h.handler.(OutboundHandler); ok {
			handlers = append(handlers, oh)
		}
	}
	return handlers
}

// LifecycleHandlers returns all LifecycleHandler handlers in the pipeline.
func (p *Pipeline) LifecycleHandlers() []LifecycleHandler {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var handlers []LifecycleHandler
	for _, h := range p.handlers {
		if lh, ok := h.handler.(LifecycleHandler); ok {
			handlers = append(handlers, lh)
		}
	}
	return handlers
}
