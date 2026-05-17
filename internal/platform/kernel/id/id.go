package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type ID string

func New(prefix string) ID {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return ID(fmt.Sprintf("%s_%d", normalizePrefix(prefix), time.Now().UnixNano()))
	}

	return ID(fmt.Sprintf("%s_%s", normalizePrefix(prefix), hex.EncodeToString(buf[:])))
}

func (i ID) String() string {
	return string(i)
}

func (i ID) Empty() bool {
	return strings.TrimSpace(string(i)) == ""
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "id"
	}

	return prefix
}
