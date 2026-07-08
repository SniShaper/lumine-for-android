// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build android && cgo

// Copied from https://github.com/golang/mobile/tree/68735029466e0b69a0c5b27f4811255254750ac3/app/android.go#L89

package platform

// #include <time.h>
import "C"
import "time"

func init() {
	var curtime C.time_t
	var curtm C.struct_tm
	C.time(&curtime)
	C.localtime_r(&curtime, &curtm)
	tzOffset := int(curtm.tm_gmtoff)
	tz := C.GoString(curtm.tm_zone)
	time.Local = time.FixedZone(tz, tzOffset)
}
