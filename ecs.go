//go:generate go run ./cmd/generate

// Package lazyecs implements a high-performance, archetype-based Entity
// Component System (ECS) for Go. It is designed for performance-critical
// applications like games and simulations, offering a simple, generic API that
// minimizes garbage collection overhead.
//
// The core of lazyecs is its archetype-based architecture. Entities with the
// same component layout are stored in contiguous memory blocks, enabling
// extremely fast iteration. This design, combined with the use of Go generics
// and unsafe pointers, provides a high-performance, type-safe, and intuitive
// developer experience.
//
// Key features include:
//   - Archetype-Based Storage: Maximizes cache efficiency by grouping entities
//     with identical component sets.
//   - Generic API: Leverages Go generics for a clean, type-safe interface.
//   - High Performance: Optimized for speed with minimal GC overhead during
//     entity creation, querying, and component access.
//   - Code Generation: Uses `go generate` to create boilerplate code for
//     handling different numbers of components, keeping the public API simple.
package lazyecs
