package attachment

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"time"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

func RenderPDF(ctx context.Context, data []byte, page int) ([]byte, error) {
	if len(data) == 0 || len(data) > MaxFile || page < 1 || page > 200 {
		return nil, errors.New("Choose PDF page 1–200 from a file up to 20 MB.")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	runtime := wazero.NewRuntimeConfig().WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling).WithMemoryLimitPages(8192).WithCloseOnContextDone(true)
	pool, err := webassembly.Init(webassembly.Config{Context: ctx, MinIdle: 0, MaxIdle: 1, MaxTotal: 1, FSConfig: wazero.NewFSConfig(), RuntimeConfig: runtime, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		return nil, errors.New("Couldn’t initialize the local PDF renderer.")
	}
	defer pool.Close()
	instance, err := pool.GetInstance(5 * time.Second)
	if err != nil {
		return nil, errors.New("The local PDF renderer is busy.")
	}
	defer instance.Close()
	document, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return nil, errors.New("This PDF is invalid, encrypted, or unsupported.")
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document.Document})
	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: document.Document})
	if err != nil || count.PageCount < 1 || count.PageCount > 200 || page > count.PageCount {
		return nil, errors.New("That page is unavailable; PDFs must contain 1–200 pages.")
	}
	rendered, err := instance.RenderPageInPixels(&requests.RenderPageInPixels{Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document.Document, Index: page - 1}}, Width: 1600, Height: 1600, RenderFlags: enums.FPDF_RENDER_FLAG_ANNOT})
	if err != nil {
		return nil, errors.New("Couldn’t render this PDF page.")
	}
	defer rendered.Cleanup()
	if rendered.Result.Width < 1 || rendered.Result.Height < 1 || rendered.Result.Width > 1600 || rendered.Result.Height > 1600 {
		return nil, errors.New("Invalid rendered page size.")
	}
	picture := rendered.Result.RenderedImage
	if rendered.Result.HasTransparency {
		opaque := image.NewRGBA(picture.Bounds())
		draw.Draw(opaque, opaque.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(opaque, opaque.Bounds(), picture, picture.Bounds().Min, draw.Over)
		picture = opaque
	}
	var output bytes.Buffer
	if err = png.Encode(&output, picture); err != nil {
		return nil, err
	}
	if output.Len() > 4*1024*1024 {
		return nil, errors.New("Rendered page exceeds 4 MB.")
	}
	return output.Bytes(), nil
}
