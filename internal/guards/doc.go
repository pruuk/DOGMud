// Package guards holds the repo-wide guard tests: tests that read the whole
// source tree or the shipped world data and fail the build when a rule the
// project depends on is broken. They scan Go source for a forbidden call or
// a second copy of a shared body, check shipped YAML content against the
// rules its loaders cannot enforce, pin goldens of the lighting model and
// the render pipeline, and hold copy rules such as the no-dash house style.
//
// None of them needs package main. They lived in the repository root until
// they were moved here; the root keeps only the tests that call code in
// package main (issue #663). The package has no production code: this file
// exists so the directory is a package. See context.md for how the tests
// find the repository root and how to run them.
package guards
