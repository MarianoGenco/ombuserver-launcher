package chat

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// HashClaveHex es la contraseña como la guarda PvPGN (acct_passhash1): el X-SHA1 de la contraseña en minúsculas,
// las cinco palabras de 32 bits en hexadecimal. La usa el permiso de voz (la API compara contra la base).
func HashClaveHex(clave string) string {
	h := hashBnet([]byte(strings.ToLower(clave)))
	var b strings.Builder
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, "%08x", binary.LittleEndian.Uint32(h[i*4:]))
	}
	return b.String()
}
