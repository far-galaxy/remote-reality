package camera

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"sort"

	"github.com/blackjack/webcam"
)

const (
	V4L2_PIX_FMT_PJPG = 0x47504A50
	V4L2_PIX_FMT_YUYV = 0x56595559
)

type FrameSizes []webcam.FrameSize

func (slice FrameSizes) Len() int {
	return len(slice)
}

// For sorting purposes
func (slice FrameSizes) Less(i, j int) bool {
	ls := slice[i].MaxWidth * slice[i].MaxHeight
	rs := slice[j].MaxWidth * slice[j].MaxHeight
	return ls < rs
}

// For sorting purposes
func (slice FrameSizes) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

var supportedFormats = map[webcam.PixelFormat]bool{
	V4L2_PIX_FMT_PJPG: true,
	V4L2_PIX_FMT_YUYV: true,
}

type Device struct {
	Cam              *webcam.Webcam
	SupportedFormats map[webcam.PixelFormat]string
	SupportedFrames  FrameSizes
	CurFormat        webcam.PixelFormat
	CurFrameSize     *string
	Width            uint32
	Height           uint32
}

func (device *Device) CheckSizeAndRequest(frameSize *string) error {
	size, err := CheckSize(frameSize, device.SupportedFrames)
	if err != nil {
		return err
	}

	fmt.Fprintln(
		os.Stderr,
		"Requesting",
		device.SupportedFormats[device.CurFormat],
		size.GetString(),
	)
	f, w, h, err := device.Cam.SetImageFormat(
		device.CurFormat,
		uint32(size.MaxWidth),
		uint32(size.MaxHeight),
	)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr,
		"Resulting image format: %v %dx%d\n",
		device.SupportedFormats[f], w, h,
	)
	device.Width = w
	device.Height = h
	device.CurFormat = f

	return nil
}

func (device *Device) PrintSupported() {
	fmt.Println("Available formats:")
	for _, s := range device.SupportedFormats {
		fmt.Fprintln(os.Stderr, s)
	}

	fmt.Fprintln(
		os.Stderr,
		"Supported frame sizes for format",
		device.SupportedFormats[device.CurFormat],
	)
	for _, f := range device.SupportedFrames {
		fmt.Fprintln(os.Stderr, f.GetString())
	}
}

func (device *Device) InitCamera(dev *string, fmtstr *string) error {
	cam, err := webcam.Open(*dev)
	if err != nil {
		return err
	}

	fmt.Printf("Selected %s\n", *dev)

	supportedFormats := cam.GetSupportedFormats()
	curFormat, err := CheckFormat(supportedFormats, fmtstr)
	if err != nil {
		return err
	}

	supportedFrames := FrameSizes(cam.GetSupportedFrameSizes(curFormat))
	sort.Sort(supportedFrames)

	device.Cam = cam
	device.SupportedFormats = supportedFormats
	device.CurFormat = curFormat
	device.SupportedFrames = supportedFrames
	return nil
}

func CheckFormat(
	supported map[webcam.PixelFormat]string,
	selected *string,
) (
	webcam.PixelFormat,
	error,
) {
	var format webcam.PixelFormat
	for f, s := range supported {
		if *selected == "" {
			if supportedFormats[f] {
				format = f
				return format, nil
			}

		} else if *selected == s {
			if !supportedFormats[f] {
				log.Println(supported[f], "format is not supported, exiting")
				return 0, fmt.Errorf("%s format is not supported, exiting", supported[f])
			}
			format = f
			return format, nil
		}
	}
	return 0, fmt.Errorf("no format found, exiting")
}

func CheckSize(szstr *string, supported FrameSizes) (*webcam.FrameSize, error) {
	var size *webcam.FrameSize
	if *szstr == "" {
		size = &supported[len(supported)-1]

		return size, nil
	} else {
		for _, f := range supported {
			if *szstr == f.GetString() {
				size = &f

				return size, nil
			}
		}
	}

	return nil, fmt.Errorf("no matching frame size, exiting")
}

func EncodeToImage(wc *webcam.Webcam, back chan struct{}, fi chan []byte, li chan *bytes.Buffer, w, h uint32, format webcam.PixelFormat) {

	var (
		frame []byte
		img   image.Image
	)
	for {
		bframe := <-fi
		// copy frame
		if len(frame) < len(bframe) {
			frame = make([]byte, len(bframe))
		}
		copy(frame, bframe)
		back <- struct{}{}

		switch format {
		case V4L2_PIX_FMT_YUYV:
			yuyv := image.NewYCbCr(image.Rect(0, 0, int(w), int(h)), image.YCbCrSubsampleRatio422)
			for i := range yuyv.Cb {
				ii := i * 4
				yuyv.Y[i*2] = frame[ii]
				yuyv.Y[i*2+1] = frame[ii+2]
				yuyv.Cb[i] = frame[ii+1]
				yuyv.Cr[i] = frame[ii+3]

			}
			img = yuyv
		default:
			log.Fatal("invalid format ?")
		}
		//convert to jpeg
		buf := &bytes.Buffer{}
		if err := jpeg.Encode(buf, img, nil); err != nil {
			log.Fatal(err)
			return
		}

		const N = 50
		// broadcast image up to N ready clients
		nn := 0
	FOR:
		for ; nn < N; nn++ {
			select {
			case li <- buf:
			default:
				break FOR
			}
		}
		if nn == 0 {
			li <- buf
		}

	}
}
