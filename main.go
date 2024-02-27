package main

import (
	"archive/zip"
	"bytes"
	"flag"
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
	var overwrite = flag.Bool("f", false, "Overwrite existing outfile.zip")
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

	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Set a threshold for buffer size to trigger flush.
	const flushThreshold = 1024 * 1024

	// Track the size of written data.
	var totalWritten int

	// Function to flush buffer to disk.
	flushBuffer := func() {
		// Write the buffer content to the output file.
		if err := os.WriteFile(*outputFile, buf.Bytes(), 0644); err != nil {
			log.Fatal(err)
		}
		// Reset the buffer and update total written size.
		buf.Reset()
		totalWritten = 0
	}

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
			// source: https://android.googlesource.com/platform/build.git/+/android-4.2.2_r1/tools/zipalign/ZipAlign.cpp#76
			newOffset := len(f.Extra) + bias
			log.Printf(" --- %s: len %d (uncompressed)", f.Name, len(f.Extra))
			padlen = (*alignment - (newOffset % *alignment)) % *alignment
			log.Printf(" --- %s: padding %d bytes %d offset", f.Name, padlen, newOffset)
			if *verbose && padlen > 0 {
				log.Printf(" --- %s: padding %d bytes", f.Name, padlen)
			}
		}

		fwhead := &zip.FileHeader{
			Name:   f.Name,
			Method: 0,
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

			n, err := rc.Read(buf)
			log.Printf("reading %d bytes", n)

			if err != nil && err != io.EOF {
				log.Fatal(err)
			}

			if n == 0 || err == io.EOF {
				break
			}

			if _, err := fw.Write(buf[:n]); err != nil {
				log.Fatal(err)
			}
			log.Printf("writing %d bytes", n)

			// Update total written size
			totalWritten += n

			// Check if we need to flush buffer to disk
			if totalWritten >= flushThreshold {
				flushBuffer()
			}

		}

		rc.Close()
		bias += padlen
	}

	// Flush the remaining buffer to disk
	if buf.Len() > 0 {
		flushBuffer()
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
