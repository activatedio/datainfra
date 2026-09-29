// Package outer is the runtime the generated outer proxy chain
// (genlib/data/outer) calls: the metadata and changelog a diff proxy hands
// its sink, the default sink, and the fx name the inner repositories go
// under.
//
// The chain wraps a flavor's repository in proxies — elapsed, validator,
// diff — each implementing the same interface and delegating inward. A
// consumer asks fx for the interface and gets the outermost proxy; the
// flavor's own implementation is registered under InnerName.
package outer
