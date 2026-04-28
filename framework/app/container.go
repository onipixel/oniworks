// Package app provides the IoC service container and application bootstrap.
package app

import (
	"fmt"
	"reflect"
	"sync"
)

// BindingFunc is a factory function that receives the container and returns a service.
type BindingFunc func(c *Container) (any, error)

// Container is the IoC service container. It manages bindings, singletons, and instances.
type Container struct {
	mu         sync.RWMutex
	bindings   map[string]BindingFunc
	singletons map[string]bool
	instances  map[string]any
	aliases    map[string]string
	tags       map[string][]string
}

// NewContainer creates a new empty Container.
func NewContainer() *Container {
	return &Container{
		bindings:   make(map[string]BindingFunc),
		singletons: make(map[string]bool),
		instances:  make(map[string]any),
		aliases:    make(map[string]string),
		tags:       make(map[string][]string),
	}
}

// Bind registers a transient binding. A new instance is created each time Make is called.
func (c *Container) Bind(abstract string, factory BindingFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[abstract] = factory
	c.singletons[abstract] = false
}

// Singleton registers a singleton binding. The same instance is returned on every Make call.
func (c *Container) Singleton(abstract string, factory BindingFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[abstract] = factory
	c.singletons[abstract] = true
}

// Instance registers an already-constructed value as a shared instance.
func (c *Container) Instance(abstract string, instance any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instances[abstract] = instance
	c.singletons[abstract] = true
}

// Alias registers an alias for an abstract binding.
func (c *Container) Alias(abstract, alias string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.aliases[alias] = abstract
}

// Tag assigns a set of abstracts to a named tag group.
func (c *Container) Tag(abstracts []string, tag string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tags[tag] = append(c.tags[tag], abstracts...)
}

// Tagged returns all services registered under the given tag.
func (c *Container) Tagged(tag string) ([]any, error) {
	c.mu.RLock()
	abstracts, ok := c.tags[tag]
	c.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	var results []any
	for _, abstract := range abstracts {
		svc, err := c.Make(abstract)
		if err != nil {
			return nil, err
		}
		results = append(results, svc)
	}
	return results, nil
}

// Make resolves a service from the container by its abstract name.
func (c *Container) Make(abstract string) (any, error) {
	// Resolve alias
	c.mu.RLock()
	if alias, ok := c.aliases[abstract]; ok {
		abstract = alias
	}
	c.mu.RUnlock()

	// Return cached singleton instance
	c.mu.RLock()
	if inst, ok := c.instances[abstract]; ok {
		c.mu.RUnlock()
		return inst, nil
	}
	factory, hasFactory := c.bindings[abstract]
	isSingleton := c.singletons[abstract]
	c.mu.RUnlock()

	if !hasFactory {
		return nil, fmt.Errorf("container: no binding registered for %q", abstract)
	}

	instance, err := factory(c)
	if err != nil {
		return nil, fmt.Errorf("container: error building %q: %w", abstract, err)
	}

	if isSingleton {
		c.mu.Lock()
		c.instances[abstract] = instance
		c.mu.Unlock()
	}

	return instance, nil
}

// MustMake resolves a service and panics on error. Useful for required dependencies.
func (c *Container) MustMake(abstract string) any {
	instance, err := c.Make(abstract)
	if err != nil {
		panic(err)
	}
	return instance
}

// MakeInto resolves a service and stores the result into dest (must be a non-nil pointer).
func (c *Container) MakeInto(abstract string, dest any) error {
	instance, err := c.Make(abstract)
	if err != nil {
		return err
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("container: dest must be a non-nil pointer, got %T", dest)
	}
	rv.Elem().Set(reflect.ValueOf(instance))
	return nil
}

// Bound reports whether an abstract has a binding (or pre-set instance).
func (c *Container) Bound(abstract string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.instances[abstract]; ok {
		return true
	}
	_, ok := c.bindings[abstract]
	return ok
}

// Flush removes all bindings and instances — useful in tests.
func (c *Container) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings = make(map[string]BindingFunc)
	c.singletons = make(map[string]bool)
	c.instances = make(map[string]any)
	c.aliases = make(map[string]string)
	c.tags = make(map[string][]string)
}
