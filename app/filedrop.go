package app

var fileDropCallback = func(files []string) {}

func FileDropCallback(fn func(files []string)) {
	fileDropCallback = fn
}
