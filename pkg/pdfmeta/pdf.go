package pdfmeta

import (
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/validate"
)

// Result holds PDF document metadata.
type Result struct {
	PageCount int
	Author    string
}

// Extract reads PDF metadata from the reader.
// Returns nil if the input is not a valid PDF or metadata extraction fails.
func Extract(r io.ReadSeeker) *Result {
	ctx, err := api.ReadContext(r, model.NewDefaultConfiguration())
	if err != nil {
		return nil
	}

	if err := validate.XRefTable(ctx); err != nil {
		return nil
	}

	return &Result{
		PageCount: ctx.PageCount,
		Author:    ctx.Author,
	}
}
