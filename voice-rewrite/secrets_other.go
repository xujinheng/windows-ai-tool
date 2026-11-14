//go:build !windows

package main

func encryptConfigSecrets(c Config) (Config, error) { return c, nil }
func decryptConfigSecrets(c Config) (Config, error) { return c, nil }
