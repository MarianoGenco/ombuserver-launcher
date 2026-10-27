package ombu

import "errors"

// Descompresor PKWARE DCL ("implode"), el que usan los MPQ originales de Diablo II (d2data.mpq, d2exp.mpq...).
// Es la traducción de blast.c (Mark Adler, en los contrib de zlib): Huffman con tablas fijas para los literales,
// los largos y las distancias, con los códigos invertidos y guardados del bit bajo al alto.

const explodeMaxBits = 13

type huffmanDCL struct {
	cuenta  [explodeMaxBits + 1]int
	simbolo []int
}

// armarHuffman arma la tabla a partir de la lista compacta de largos (cada byte: repeticiones-1 en el nibble alto,
// largo en el bajo).
func armarHuffman(rep []byte) *huffmanDCL {
	var largos []int
	for _, b := range rep {
		for k := int(b>>4) + 1; k > 0; k-- {
			largos = append(largos, int(b&15))
		}
	}
	h := &huffmanDCL{simbolo: make([]int, len(largos))}
	for _, l := range largos {
		h.cuenta[l]++
	}
	var offs [explodeMaxBits + 2]int
	for l := 1; l < explodeMaxBits; l++ {
		offs[l+1] = offs[l] + h.cuenta[l]
	}
	for s, l := range largos {
		if l != 0 {
			h.simbolo[offs[l]] = s
			offs[l]++
		}
	}
	return h
}

var (
	dclLiterales = armarHuffman([]byte{11, 124, 8, 7, 28, 7, 188, 13, 76, 4, 10, 8, 12, 10, 12, 10, 8, 23, 8, 9, 7, 6, 7,
		8, 7, 6, 55, 8, 23, 24, 12, 11, 7, 9, 11, 12, 6, 7, 22, 5, 7, 24, 6, 11, 9, 6, 7, 22, 7, 11, 38, 7, 9, 8, 25, 11,
		8, 11, 9, 12, 8, 12, 5, 38, 5, 38, 5, 11, 7, 5, 6, 21, 6, 10, 53, 8, 7, 24, 10, 27, 44, 253, 253, 253, 252, 252,
		252, 13, 12, 45, 12, 45, 12, 61, 12, 45, 44, 173})
	dclLargos     = armarHuffman([]byte{2, 35, 36, 53, 38, 23})
	dclDistancias = armarHuffman([]byte{2, 20, 53, 230, 247, 151, 248})
	dclBaseLargo  = [16]int{3, 2, 4, 5, 6, 7, 8, 9, 10, 12, 16, 24, 40, 72, 136, 264}
	dclExtraLargo = [16]uint{0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}
)

var errExplode = errors.New("datos PKWARE inválidos")

type lectorBits struct {
	in   []byte
	pos  int
	buf  uint32
	nBit uint
}

func (r *lectorBits) bits(n uint) (int, error) {
	for r.nBit < n {
		if r.pos >= len(r.in) {
			return 0, errExplode
		}
		r.buf |= uint32(r.in[r.pos]) << r.nBit
		r.pos++
		r.nBit += 8
	}
	v := int(r.buf & (1<<n - 1))
	r.buf >>= n
	r.nBit -= n
	return v, nil
}

func (r *lectorBits) decodificar(h *huffmanDCL) (int, error) {
	codigo, primero, indice := 0, 0, 0
	for l := 1; l <= explodeMaxBits; l++ {
		b, err := r.bits(1)
		if err != nil {
			return 0, err
		}
		codigo |= b ^ 1 // los códigos van invertidos
		n := h.cuenta[l]
		if codigo < primero+n {
			return h.simbolo[indice+codigo-primero], nil
		}
		indice += n
		primero = (primero + n) << 1
		codigo <<= 1
	}
	return 0, errExplode
}

// explotar descomprime un bloque PKWARE DCL; esperado es el tamaño descomprimido (el MPQ lo sabe).
func explotar(in []byte, esperado int) ([]byte, error) {
	r := &lectorBits{in: in}
	lit, err := r.bits(8)
	if err != nil || lit > 1 {
		return nil, errExplode
	}
	dic, err := r.bits(8)
	if err != nil || dic < 4 || dic > 6 {
		return nil, errExplode
	}
	out := make([]byte, 0, esperado)
	for len(out) < esperado {
		marca, err := r.bits(1)
		if err != nil {
			return nil, err
		}
		if marca == 0 { // literal
			var s int
			if lit == 1 {
				s, err = r.decodificar(dclLiterales)
			} else {
				s, err = r.bits(8)
			}
			if err != nil {
				return nil, err
			}
			out = append(out, byte(s))
			continue
		}
		s, err := r.decodificar(dclLargos) // copia de lo anterior: largo y distancia
		if err != nil {
			return nil, err
		}
		extra, err := r.bits(dclExtraLargo[s])
		if err != nil {
			return nil, err
		}
		largo := dclBaseLargo[s] + extra
		if largo == 519 { // fin
			break
		}
		corrimiento := uint(dic)
		if largo == 2 {
			corrimiento = 2
		}
		d, err := r.decodificar(dclDistancias)
		if err != nil {
			return nil, err
		}
		bajos, err := r.bits(corrimiento)
		if err != nil {
			return nil, err
		}
		dist := d<<corrimiento + bajos + 1
		if dist > len(out) {
			return nil, errExplode
		}
		for i := 0; i < largo && len(out) < esperado; i++ {
			out = append(out, out[len(out)-dist])
		}
	}
	if len(out) != esperado {
		return nil, errors.New("sector PKWARE de tamaño distinto al esperado")
	}
	return out, nil
}
