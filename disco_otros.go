//go:build !windows

package main

func espacioLibre(dir string) (uint64, bool) { return 0, false }
