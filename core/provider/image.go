package provider

import "errors"

type Image struct {
	MIME string
	Data []byte
}

func (image Image) Validate() error {
	if image.MIME != "image/png" && image.MIME != "image/jpeg" && image.MIME != "image/webp" {
		return errors.New("unsupported image type")
	}
	if len(image.Data) == 0 || len(image.Data) > 4*1024*1024 {
		return errors.New("image must contain 1 byte to 4 MB")
	}
	return nil
}
