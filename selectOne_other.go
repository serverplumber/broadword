//go:build !amd64 || purego

// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

func selectOne(x uint64, n int) int {
	return genericSelectOne(x, n)
}
