package remotevm

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"strconv"
)

func readPPM(in io.Reader) (image.Image, error) {
	buf := bufio.NewReader(in)
	var err error

	header := make([]byte, 0)
	comment := false
	var b byte
	for fields := 0; fields < 4; {
		b, err = buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == '#' {
			comment = true
		} else if !comment {
			header = append(header, b)
		}
		if comment && b == '\n' {
			comment = false
		} else if !comment && (b == ' ' || b == '\n' || b == '\t') {
			fields++
		}
	}
	headerFields := bytes.Fields(header)
	magicNumber := string(headerFields[0])
	if magicNumber != "P6" {
		return nil, fmt.Errorf("Expecting magic P6, got %s", magicNumber)
	}
	width, err := strconv.Atoi(string(headerFields[1]))
	if err != nil {
		return nil, errors.New("Failed to decoded picture width")
	}
	if width < 1 || width > 10000 {
		return nil, fmt.Errorf("Abnormal image width %d", width)
	}
	height, err := strconv.Atoi(string(headerFields[2]))
	if err != nil {
		return nil, errors.New("Failed to decoded picture height")
	}
	if height < 1 || height > 10000 {
		return nil, fmt.Errorf("Abnormal image height %d", height)
	}
	maxBitmapVal, err := strconv.Atoi(string(headerFields[3]))
	if err != nil {
		return nil, errors.New("Failed to decide maximum bitmap value")
	} else if maxBitmapVal != 255 {
		return nil, fmt.Errorf("Unsupported maximum bitmap value %d", maxBitmapVal)
	}

	pixel := make([]byte, 3)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			_, err = io.ReadFull(buf, pixel)
			if err != nil {
				return nil, err
			}
			img.SetRGBA(x, y, color.RGBA{pixel[0], pixel[1], pixel[2], 0xff})
		}
	}
	return img, nil
}
