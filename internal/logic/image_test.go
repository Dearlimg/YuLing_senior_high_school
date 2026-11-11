package logic

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"math/rand/v2"
	"testing"
)

func TestNoisyImageIsAlwaysBoundedBeforeStorage(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 1400, 1400))
	random := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < len(source.Pix); i += 4 {
		source.Pix[i], source.Pix[i+1], source.Pix[i+2], source.Pix[i+3] = byte(random.Uint32()), byte(random.Uint32()), byte(random.Uint32()), 255
	}
	var original bytes.Buffer
	if err := jpeg.Encode(&original, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if original.Len() > MaxUploadImageBytes || original.Len() <= MaxStoredImageBytes {
		t.Fatal("test image must fit upload but exceed storage limit")
	}
	data, err := processImage(original.Bytes(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > MaxStoredImageBytes {
		t.Fatal("oversized stored image")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" || max(config.Width, config.Height) > MaxImageDimension {
		t.Fatal("invalid normalized image")
	}
}

func TestImageBytesAndDecodedPixelsAreCheckedBeforeDecode(t *testing.T) {
	if _, err := processImage(make([]byte, MaxUploadImageBytes+1), true); !errors.Is(err, ErrImageTooLarge) {
		t.Fatal("oversized image accepted")
	}
	png := testPNG(t)
	binary.BigEndian.PutUint32(png[16:20], 100000)
	binary.BigEndian.PutUint32(png[20:24], 100000)
	binary.BigEndian.PutUint32(png[29:33], crc32.ChecksumIEEE(png[12:29]))
	if _, err := processImage(png, true); !errors.Is(err, ErrInvalidParam) {
		t.Fatal("decoded pixel limit bypassed")
	}
}
