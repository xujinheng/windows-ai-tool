//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// Windows DPAPI: secrets are tied to the current Windows user, not to an
// application-specific encryption key that could accidentally be committed.
const dpapiPrefix = "dpapi:v1:"

type dataBlob struct {
	Size uint32
	Data *byte
}

var (
	crypt32            = syscall.NewLazyDLL("crypt32.dll")
	cryptProtectData   = crypt32.NewProc("CryptProtectData")
	cryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	cryptLocalFree     = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")
)

func dpapiTransform(data []byte, protect bool) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	in := dataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out dataBlob
	var ok uintptr
	var err error
	if protect {
		// CRYPTPROTECT_UI_FORBIDDEN; don't open OS dialogs on a background save.
		ok, _, err = cryptProtectData.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	} else {
		ok, _, err = cryptUnprotectData.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	}
	runtime.KeepAlive(data)
	if ok == 0 {
		return nil, fmt.Errorf("DPAPI: %w", err)
	}
	defer cryptLocalFree.Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func encryptSecret(s string) (string, error) {
	if s == "" || strings.HasPrefix(s, dpapiPrefix) {
		return s, nil
	}
	raw, err := dpapiTransform([]byte(s), true)
	if err != nil {
		return "", err
	}
	return dpapiPrefix + base64.StdEncoding.EncodeToString(raw), nil
}
func decryptSecret(s string) (string, error) {
	if !strings.HasPrefix(s, dpapiPrefix) {
		return s, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, dpapiPrefix))
	if err != nil {
		return "", fmt.Errorf("密钥编码错误: %w", err)
	}
	plain, err := dpapiTransform(raw, false)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
func encryptConfigSecrets(c Config) (Config, error) {
	var err error
	if c.ASRAPIKey, err = encryptSecret(c.ASRAPIKey); err != nil {
		return c, err
	}
	if c.LLMAPIKey, err = encryptSecret(c.LLMAPIKey); err != nil {
		return c, err
	}
	return c, nil
}
func decryptConfigSecrets(c Config) (Config, error) {
	var err error
	if c.ASRAPIKey, err = decryptSecret(c.ASRAPIKey); err != nil {
		return c, err
	}
	if c.LLMAPIKey, err = decryptSecret(c.LLMAPIKey); err != nil {
		return c, err
	}
	return c, nil
}
