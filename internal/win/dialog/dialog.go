package dialog

import "golang.org/x/sys/windows"

// MessageBox shows a native Windows message dialog.
func MessageBox(title, text string, isError bool) {
	flags := uint32(windows.MB_OK)
	if isError {
		flags |= windows.MB_ICONERROR
	} else {
		flags |= windows.MB_ICONINFORMATION
	}
	windows.MessageBox(0, windows.StringToUTF16Ptr(text), windows.StringToUTF16Ptr(title), flags)
}
