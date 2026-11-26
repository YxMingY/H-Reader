//go:build darwin && !ios && cgo

package book

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static int copyScreenshotPNG(const void *bytes, size_t length) {
	@autoreleasepool {
		NSData *data = [NSData dataWithBytes:bytes length:length];
		NSPasteboardItem *item = [[[NSPasteboardItem alloc] init] autorelease];
		if (![item setData:data forType:NSPasteboardTypePNG]) {
			return NO;
		}
		NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
		[pasteboard clearContents];
		return [pasteboard writeObjects:@[item]];
	}
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func copyScreenshotPNG(png []byte) error {
	return application.InvokeSyncWithError(func() error {
		if C.copyScreenshotPNG(unsafe.Pointer(&png[0]), C.size_t(len(png))) == 0 {
			return errors.New("macOS 剪贴板写入失败")
		}
		return nil
	})
}
