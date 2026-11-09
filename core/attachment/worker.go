package attachment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"time"
)

func Worker(input io.Reader, output io.Writer) error {
	debug.SetMemoryLimit(256 * 1024 * 1024)
	data, err := io.ReadAll(io.LimitReader(input, MaxFile+1))
	if err != nil || len(data) > MaxFile {
		return errors.New("PDF input exceeds limit")
	}
	pages, err := PDF(data)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(pages)
}

type limitedOutput struct{ bytes.Buffer }

func (w *limitedOutput) Write(data []byte) (int, error) {
	if w.Len()+len(data) > 4*1024*1024 {
		return 0, errors.New("PDF extraction output exceeds limit")
	}
	return w.Buffer.Write(data)
}
func ExtractPDFWorker(ctx context.Context, data []byte) ([]Page, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "--extract-pdf")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "SystemRoot=" + os.Getenv("SystemRoot"), "TMPDIR=" + os.TempDir()}
	command.Stdin = bytes.NewReader(data)
	var output limitedOutput
	command.Stdout = &output
	if err = command.Run(); err != nil {
		return nil, errors.New("PDF extraction failed or exceeded its processing limit.")
	}
	var pages []Page
	if err = json.Unmarshal(output.Bytes(), &pages); err != nil {
		return nil, errors.New("PDF extraction returned invalid data.")
	}
	return pages, nil
}

func RenderWorker(input io.Reader, output io.Writer, page int) error {
	debug.SetMemoryLimit(512 * 1024 * 1024)
	data, err := io.ReadAll(io.LimitReader(input, MaxFile+1))
	if err != nil || len(data) > MaxFile {
		return errors.New("PDF input exceeds limit")
	}
	image, err := RenderPDF(context.Background(), data, page)
	if err != nil {
		return err
	}
	_, err = output.Write(image)
	return err
}

var renderSlots = make(chan struct{}, 2)

func RenderPDFWorker(ctx context.Context, data []byte, page int) ([]byte, error) {
	if len(data) == 0 || len(data) > MaxFile || page < 1 || page > 200 {
		return nil, errors.New("Invalid PDF page request.")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	select {
	case renderSlots <- struct{}{}:
		defer func() { <-renderSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, executable, "--render-pdf", strconv.Itoa(page))
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "SystemRoot=" + os.Getenv("SystemRoot"), "TMPDIR=" + os.TempDir()}
	command.Stdin = bytes.NewReader(data)
	var output limitedOutput
	command.Stdout = &output
	if err = command.Run(); err != nil {
		return nil, errors.New("PDF rendering failed or exceeded its processing limit.")
	}
	config, err := png.DecodeConfig(bytes.NewReader(output.Bytes()))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 1600 || config.Height > 1600 {
		return nil, errors.New("PDF renderer returned an invalid page image.")
	}
	return output.Bytes(), nil
}
