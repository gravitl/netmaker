package expr

import (
	"fmt"
)

type SearchField struct {
	Field     string
	Formatter func(quoted string) string
}

var ByteaIPFormatter = func(quoted string) string {
	if CurrentDialect() == DialectPostgres {
		return fmt.Sprintf(
			"(get_byte(%s, 12)::text || '.' || get_byte(%s, 13)::text || '.' || get_byte(%s, 14)::text || '.' || get_byte(%s, 15)::text)",
			quoted, quoted, quoted, quoted,
		)
	}
	return fmt.Sprintf("hex(%s)", quoted)
}

func ByteaField(field string) SearchField {
	return SearchField{
		Field:     field,
		Formatter: ByteaIPFormatter,
	}
}
