#include "textflag.h"

// -- Kernel de dot int8 com AVX2 -- L339 ------------------------------------
// 16 elementos por iteracao (1 byte/elem = 4x menos bandwidth que float32):
//   VMOVDQU (16B) + VPMOVZXBW (qb unsigned) + VPMOVSXBW (rb signed)
//   + VPMADDWD (8x i16 -> 4x i32) + VPADDD no acumulador.
//
// RESTRICOES DO GO ASSEMBLER 1.26.5 (L339, bugs validados):
//   - VPMADDUBSW: modrm.reg sai errado para certas combinacoes (evitado).
//   - VPMOVZXBW/SXBW: so XMM (nao YMM) e so reg-reg (sem memoria).
//   - Registradores XMM/YMM >= 8: bit R' codificado errado (evitado).
// Logo: apenas X0-X7, extends reg-reg, VPMADDWD/VPADDD (validados).
//
// Aritmetica de bias: qb = q+128 (uint8, UNSIGNED) x rb = r (int8, SIGNED)
// logo Soma(qb*rb) = Soma(q*r) + 128*Soma(r). A correcao (-128*sumR) e feita
// pelo chamador (1 mul + 1 sub por row - custo imperceptivel).
//
// ABI: abi0 (stack args: qb+0, rb+8, n+16; ret em ret+24). O caller Go
// (dot8BiasDot) chama via ABI0 padrao do compilador para stubs assembly.

// func dot8BiasAVX2(qb *uint8, rb *int8, n int) uint32
// Retorna Soma(qb[i]*rb[i]) com acumulacao i32 (n <= 16384: teto
// 127*127*16384 ~= 264M, abaixo de 2^31). Requer n % 16 == 0.
TEXT ·dot8BiasAVX2(SB), NOSPLIT, $0-28
	MOVQ qb+0(FP), DI
	MOVQ rb+8(FP), SI
	MOVQ n+16(FP), DX

	VPXOR X0, X0, X0            // acumulador 4x i32
	MOVQ DX, AX
	SHRQ $4, AX                 // iteracoes = n/16
	TESTQ AX, AX
	JZ   done

loop:
	VMOVDQU 0(DI), X1           // qb[0:16]
	VMOVDQU 8(DI), X2           // qb[8:24]
	VMOVDQU 0(SI), X3           // rb[0:16]
	VMOVDQU 8(SI), X4           // rb[8:24]

	VPMOVZXBW X1, X5            // X5 <- zx(qb[0:8])   (src, dst)
	VPMOVZXBW X2, X6            // X6 <- zx(qb[8:16])
	VPMOVSXBW X3, X1            // X1 <- sx(rb[0:8])   (reuso X1)
	VPMOVSXBW X4, X2            // X2 <- sx(rb[8:16])  (reuso X2)

	VPMADDWD X1, X5, X5         // X5 <- X5*X1         (src2, src1, dst)
	VPMADDWD X2, X6, X6         // X6 <- X6*X2

	VPADDD X0, X5, X0           // X0 <- X5+X0         (src2, src1, dst)
	VPADDD X0, X6, X0

	ADDQ $16, DI
	ADDQ $16, SI
	DECQ AX
	JNZ  loop

done:
	VPSHUFD $0xEE, X0, X1       // 4->2 i32
	VPADDD X0, X1, X0
	VPSHUFD $0x55, X0, X1       // 2->1
	VPADDD X0, X1, X0
	VMOVD X0, AX
	MOVL AX, ret+24(FP)
	RET
