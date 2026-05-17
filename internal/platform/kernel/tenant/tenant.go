package tenant

import "strings"

type ID string

func (i ID) String() string {
	return string(i)
}

func (i ID) Empty() bool {
	return strings.TrimSpace(string(i)) == ""
}
