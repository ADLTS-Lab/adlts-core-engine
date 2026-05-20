package reporting

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/url"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)
//go:embed report_template.html
var reportTemplateFS embed.FS

type Renderer struct {
	tmpl *template.Template
}

func NewRenderer() (*Renderer, error) {
	parsed, err := template.ParseFS(reportTemplateFS, "report_template.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{tmpl: parsed}, nil
}

func (r *Renderer) RenderHTML(data ReportData) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func RenderPDF(ctx context.Context, html []byte) ([]byte, error) {
	allocatorCtx, cancel := chromedp.NewExecAllocator(ctx, chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancel()

	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	var pdfBuf []byte
	dataURL := "data:text/html," + url.PathEscape(string(html))
	tasks := chromedp.Tasks{
		chromedp.Navigate(dataURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().WithPrintBackground(true).WithPaperWidth(8.27).WithPaperHeight(11.69).WithMarginTop(0.5).WithMarginBottom(0.5).WithMarginLeft(0.5).WithMarginRight(0.5).Do(ctx)
			if err != nil {
				return err
			}
			pdfBuf = buf
			return nil
		}),
	}
	if err := chromedp.Run(browserCtx, tasks); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return pdfBuf, nil
}
