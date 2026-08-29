// Package bringup defines the marker that orders database bring-up — setup
// and migrations — before anything opens a pooled connection. The gorm
// composition lives in bringup/gorm; the marker is dialect-agnostic so other
// flavors can share it.
package bringup
