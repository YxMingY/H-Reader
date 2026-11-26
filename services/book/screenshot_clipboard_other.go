//go:build !darwin || ios || !cgo

package book

import "errors"

func copyScreenshotPNG([]byte) error {
	return errors.New("当前平台不支持此图片剪贴板接口")
}
