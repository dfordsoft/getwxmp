package main

import (
	"errors"
	"os"

	pdf "github.com/unidoc/unidoc/pdf/model"
)

func mergePDFDocument(inputPath string, pdfWriter *pdf.PdfWriter) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return err
	}

	defer f.Close()

	pdfReader, err := pdf.NewPdfReader(f)
	if err != nil {
		return err
	}

	isEncrypted, err := pdfReader.IsEncrypted()
	if err != nil {
		return err
	}

	if isEncrypted {
		auth, err := pdfReader.Decrypt([]byte(""))
		if err != nil {
			return err
		}
		if !auth {
			return errors.New("Cannot merge encrypted, password protected document")
		}
	}

	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return err
	}

	for i := 0; i < numPages; i++ {
		pageNum := i + 1

		page, err := pdfReader.GetPage(pageNum)
		if err != nil {
			return err
		}

		err = pdfWriter.AddPage(page)
		if err != nil {
			return err
		}
	}

	return nil
}

func mergePDFs(inputPaths []string, outputPath string) error {
	pdfWriter := pdf.NewPdfWriter()

	for _, inputPath := range inputPaths {
		if err := mergePDFDocument(inputPath, &pdfWriter); err != nil {
			return err
		}
	}

	fWrite, err := os.Create(outputPath)
	if err != nil {
		return err
	}

	defer fWrite.Close()

	err = pdfWriter.Write(fWrite)
	if err != nil {
		return err
	}

	return nil
}
