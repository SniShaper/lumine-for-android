//go:build android && cgo

package mobile

/*
#cgo LDFLAGS: -ldl
#include <android/fdsan.h>
#include <dlfcn.h>

static void lumine_fdsan_set_level(int level) {
	void (*fn)(int) = (void (*)(int))dlsym(RTLD_DEFAULT, "android_fdsan_set_error_level");
	if (fn != 0) {
		fn(level);
	}
}
*/
import "C"

//export SetFdsanWarnOnly
func SetFdsanWarnOnly(warnOnly bool) {
	level := C.ANDROID_FDSAN_ERROR_LEVEL_FATAL
	if warnOnly {
		level = C.ANDROID_FDSAN_ERROR_LEVEL_WARN_ALWAYS
	}
	C.lumine_fdsan_set_level(C.int(level))
}
