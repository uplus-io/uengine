/*
 * Copyright (c) 2019 uplus.io
 */

package uengine

import "errors"

//database
var (
	ErrPartRingVerifyFailed = errors.New("partition ring verify failed")
	ErrPartNotFound         = errors.New("partition not found")
	ErrPartAllocated        = errors.New("partition has been allocated")
	ErrPartNotAllocate      = errors.New("partition not allocate")
)

var (
	ErrDbKeyNotFound = errors.New("db:key not found")
)