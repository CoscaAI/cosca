#include "textflag.h"

// -- Kernel de dot int16 com AVX2 -- FASE 7 (L347) --------------------------
// 32 elementos por iteracao (2 bytes/elem = 2x menos bandwidth que float32):
//   VMOVDQU YMM (16 i16 cada) + VPMADDWD (16 pares -> 8 i32) + VPADDD YMM.
// SEM bias (diferente do int8): o produto i16xi16 cabe em i32 com escala 2^10.
//
// Escala: q16 = round(q*1024), r16 = round(r*1024). Par max = 2*1024^2 =
// 8.4M; acumulador i32 com 8 lanes: dim <= 1024 -> max por lane =
// (dim/8)*8.4M <= 2.1e9 (sem estouro). Resolucao: 11 bits (16x melhor que
// int8 7 bits).
//
// ABI: abi0 (stack args: a+0, b+8, n+16; ret em ret+24). Ordem plan9:
// 2-op = (src, dst), 3-op = (src2, src1, dst) — validado por execucao (L339).

// func dot16AVX2(a *int16, b *int16, n int) int32
// Retorna Soma(a[i]*b[i]) com acumulacao i32. Requer n % 32 == 0, n > 0.
TEXT ·dot16AVX2(SB), NOSPLIT, $0-28
	MOVQ a+0(FP), DI
	MOVQ b+8(FP), SI
	MOVQ n+16(FP), DX

	VPXOR Y0, Y0, Y0            // acumulador 8x i32
	MOVQ DX, AX
	SHRQ $5, AX                 // iteracoes = n/32
	TESTQ AX, AX
	JZ   done

loop:
	VMOVDQU 0(DI), Y1           // q[0:16]   (16 i16 = 32 bytes)
	VMOVDQU 0(SI), Y2           // r[0:16]
	VPMADDWD Y1, Y2, Y3         // 8 i32     (src2, src1, dst)
	VPADDD Y0, Y3, Y0           // acumula   (src2, src1, dst)
	VMOVDQU 32(DI), Y1          // q[16:32]
	VMOVDQU 32(SI), Y2
	VPMADDWD Y1, Y2, Y3
	VPADDD Y0, Y3, Y0
	ADDQ $64, DI
	ADDQ $64, SI
	DECQ AX
	JNZ  loop

done:
	VEXTRACTI128 $1, Y0, X1     // X1 = 4 i32 altos
	VPADDD X0, X1, X0           // X0 = 4 i32 (soma das lanes)
	VPSHUFD $0xEE, X0, X1       // 4 -> 2
	VPADDD X0, X1, X0
	VPSHUFD $0x55, X0, X1       // 2 -> 1
	VPADDD X0, X1, X0
	VMOVD X0, AX
	MOVL AX, ret+24(FP)
	RET
