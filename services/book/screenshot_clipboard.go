package book

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
)

func (a *BookService) CopyScreenshotPNG(dataURL string) error {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return errors.New("截图不是 PNG 图片")
	}
	png, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil || !bytes.HasPrefix(png, []byte("\x89PNG\r\n\x1a\n")) {
		return errors.New("无法读取截图数据")
	}
	return copyScreenshotPNG(png)
}
