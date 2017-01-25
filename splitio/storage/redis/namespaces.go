// Package redis implements different kind of storages for split information
package redis

import (
	"fmt"
	"strings"
)

const _splitNamespace = "SPLITIO.split.%s"
const _splitsTillNamespace = "SPLITIO.splits.till"
const _segmentsRegisteredNamespace = "SPLITIO.segments.registered"

type prefixAdapter struct {
	prefix string
}

func (p prefixAdapter) setPrefixPattern(pattern string) string {
	if p.prefix != "" {
		return strings.Join([]string{p.prefix, pattern}, ".")
	}
	return pattern
}

func (p prefixAdapter) splitNamespace(name string) string {
	return fmt.Sprintf(p.setPrefixPattern(_splitNamespace), name)
}

func (p prefixAdapter) splitsTillNamespace() string {
	return fmt.Sprint(p.setPrefixPattern(_splitsTillNamespace))
}

func (p prefixAdapter) segmentsRegisteredNamespace() string {
	return fmt.Sprint(p.setPrefixPattern(_segmentsRegisteredNamespace))
}
