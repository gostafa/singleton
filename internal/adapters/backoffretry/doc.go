// Package backoffretry adapts github.com/cenkalti/backoff to the singleton
// Retrier port.
//
// It is the only package that imports a retry engine. Everything the engine
// reports is translated into domain types here, so no part of the core — and
// no caller — depends on its error model.
package backoffretry
