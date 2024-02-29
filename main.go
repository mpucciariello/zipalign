package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	var bias int
	var verbose = flag.Bool("v", false, "Verbose output")
	var alignment = flag.Int("a", 4, "Alignment in bytes, e.g. '4' provides 32-bit alignment")
	var inputFile = flag.String("i", "bdt.v68.dat", "Input ZIP file to be aligned")
	var outputFile = flag.String("o", "bdt.v68.aligned.dat", "Output aligned ZIP file")
	var overwrite = flag.Bool("f", true, "Overwrite existing outfile.zip")
	var help = flag.Bool("h", false, "Print this help")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" || *help {
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *inputFile == *outputFile && !*overwrite {
		log.Fatalf("Refusing to overwrite output file %q without -f being set", *outputFile)
	}
	if *verbose {
		log.Printf("Aligning %q on %d bytes and writing out to %q", *inputFile, *alignment, *outputFile)
	}

	// Open a zip archive for reading.
	r, err := zip.OpenReader(*inputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	zipf, _ := os.Create(*outputFile)
	defer zipf.Close()

	// Create a new zip archive.
	w := zip.NewWriter(zipf)

	// Track the size of written data.
	var totalWritten int

	// Iterate through the files in the archive.
	for _, f := range r.File {
		if *verbose {
			log.Printf("Processing %q from input archive", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}

		var padlen int
		if f.CompressedSize64 != f.UncompressedSize64 {
			// File is compressed, copy the entry without padding
			if *verbose {
				log.Printf("--- %s: len %d (compressed)", f.Name, f.UncompressedSize64)
			}
		} else {
			newOffset := len(f.Extra) + bias
			log.Printf(" --- %s: len %d (uncompressed)", f.Name, len(f.Extra))
			padlen = (*alignment - (newOffset % *alignment)) % *alignment
			log.Printf(" --- %s: padding %d bytes %d offset", f.Name, padlen, newOffset)
			if *verbose && padlen > 0 {
				log.Printf(" --- %s: padding %d bytes", f.Name, padlen)
			}
		}

		fwhead := &zip.FileHeader{
			Name:               f.Name,
			Method:             f.Method,
			UncompressedSize64: f.UncompressedSize64,
		}
		// add padlen number of null bytes to the extra field of the file header
		// in order to align files on 4 bytes
		for i := 0; i < padlen; i++ {
			fwhead.Extra = append(fwhead.Extra, '\x00')
		}

		fw, err := w.CreateHeader(fwhead)
		if err != nil {
			log.Fatal(err)
		}

		buf := make([]byte, 1024*1024)

		for {
			// 1MB buffer
			n, err := rc.Read(buf)

			if err != nil && err != io.EOF {
				log.Fatal(err)
			}

			if n == 0 || err == io.EOF {
				break
			}

			if _, err := fw.Write(buf[:n]); err != nil {
				log.Fatal(err)
			}

			// Update total written size
			totalWritten += n

		}

		bias += padlen
		rc.Close()
	}

	// Close the zip writer after writing all the content
	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}

	zipFilePath := "bdt.v68.dat"

	zipFile, err := zip.OpenReader(zipFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer zipFile.Close()

	for _, file := range zipFile.File {
		fileReader, err := file.Open()
		if err != nil {
			log.Println("Error al abrir el archivo", file.Name, ":", err)
			continue
		}
		defer fileReader.Close()
		fmt.Println(file.Name, file.FileInfo().Size(), file.CRC32)
	}

	lenZip()

}

func lenZip() {
	zipFilePath := "bdt.v68.aligned.dat"

	zipFile, err := zip.OpenReader(zipFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer zipFile.Close()

	for _, file := range zipFile.File {
		fileReader, err := file.Open()
		if err != nil {
			log.Println("Error al abrir el archivo", file.Name, ":", err)
			continue
		}
		defer fileReader.Close()
		fmt.Println(file.Name, file.FileInfo().Size(), file.CRC32)
		if len(file.Extra)%4 == 0 {

			fmt.Println("El archivo", file.Name, "está alineado.")
		} else {
			fmt.Println("El archivo", file.Name, "NO está alineado.")
		}
	}
}
