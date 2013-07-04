package main

import "fmt"
import "image"
import "image/draw"
import "image/color"
import "os"
import "strings"
import "image/png"
import _ "image/gif"
import _ "image/jpeg"

type IntRange struct {
	start, end int
}

func main() {
	// Get the one (and only one) argument to the program, which is the filename we want to look at.
	// If we don't get it, complain.

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "We received %d arguments when we expected only one: the file to process.\n", len(os.Args)-1)

		os.Exit(1)
	} else if strings.HasPrefix(os.Args[1], "-h") || strings.HasPrefix(os.Args[1], "--help") {
		printHelp()

		os.Exit(0)
	}

	// So we should have a filename, ensure we can read it

	filename := os.Args[1]

	fmt.Printf("Opening %s...\n", filename)

	file, err := os.OpenFile(filename, os.O_RDONLY, 0)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open file %s: %s\n", filename, err)

		os.Exit(2)
	}

	// Now that our file is open, let's be sure we close it if something else goes wrong

	defer file.Close()

	// Now we can do our actual work. Let's get our image.

	fmt.Println("Decoding image...")

	inputImage, _, err := image.Decode(file)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error trying to read your image: %s", err)

		os.Exit(3)
	}

	// Let's put that in an RGBA so we can easily mess with it

	fmt.Println("Making a copy for us image...")

	ourImage := image.NewRGBA(inputImage.Bounds())

	draw.Draw(ourImage, ourImage.Bounds(), inputImage, image.Point{0, 0}, draw.Src)

	// Clamp all the pixel values

	fmt.Println("Clamping pixels...")

	clampPixels(ourImage)

	// Time to start working. We need to divide things into row ranges for processing.

	fmt.Println("Detecting empty rows...")

	rowRanges := findRowRanges(ourImage)

	nextSequenceNumber := 1

	for _, rowRange := range rowRanges {
		// Process that set of rows. The number of images created will be returned.

		fmt.Printf("\tProcessing rows %d to %d...\n", rowRange.start, rowRange.end)

		nextSequenceNumber = processRow(ourImage, rowRange, nextSequenceNumber)
	}

	// That's it

	fmt.Printf("%d images extracted.\n", nextSequenceNumber-1)
}

func processRow(inputImage *image.RGBA, rowRange IntRange, sequenceNumber int) int {
	// Figure out the column ranges

	fmt.Printf("\t\tFinding individual images...\n", rowRange.start, rowRange.end)

	columnRanges := findColumnRanges(inputImage, rowRange.start, rowRange.end)

	for _, columnRange := range columnRanges {
		extractImage(inputImage, rowRange, columnRange, sequenceNumber)

		sequenceNumber++
	}

	return sequenceNumber
}

func clampPixels(inputImage *image.RGBA) {
	bounds := inputImage.Bounds()

	// Force all pixels to black or white

	for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
		for x := bounds.Min.X; x <= bounds.Max.X; x++ {
			if colorIsWhite(inputImage.At(x, y)) {
				inputImage.Set(x, y, color.White)
			} else {
				inputImage.Set(x, y, color.Black)
			}
		}
	}
}

func extractImage(inputImage *image.RGBA, rowRange IntRange, columnRange IntRange, sequenceNumber int) {
	// Make a new image from the selected portion of the original image

	ourRectangle := image.Rect(columnRange.start, rowRange.start, columnRange.end, rowRange.end)

	subImage := inputImage.SubImage(ourRectangle)

	// Save that to disk in PNG format

	writePNG(subImage, sequenceNumber)

	// And now in C format

	writeC(subImage, sequenceNumber)
}

func writePNG(image image.Image, sequenceNumber int) {
	filename := fmt.Sprintf("%d.png", sequenceNumber)

	fmt.Printf("\t\t\tWriting %s, which is %dx%d...\n", filename, image.Bounds().Dx(), image.Bounds().Dy())

	outputFile, err := os.Create(filename)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to open file %s: %s\n", filename, err)

		os.Exit(4)
	}

	defer outputFile.Close()

	err = png.Encode(outputFile, image)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to write PNG %s: %s\n", filename, err)

		os.Exit(5)
	}
}

func writeC(image image.Image, sequenceNumber int) {
	filename := fmt.Sprintf("%d.c", sequenceNumber)

	fmt.Printf("\t\t\tWriting %s as C data...\n", filename)

	outputFile, err := os.Create(filename)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to open file %s: %s\n", filename, err)

		os.Exit(6)
	}

	defer outputFile.Close()

	// Figure out how many bytes each row will take up

	rowInBytes := image.Bounds().Dx() / 8

	if image.Bounds().Dx()%8 != 0 {
		rowInBytes++
	}

	// First we'll write out the start of the file

	_, err = fmt.Fprintf(outputFile, "byte image%dWidth %d;\n", sequenceNumber, image.Bounds().Dx())

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

		os.Exit(7)
	}

	_, err = fmt.Fprintf(outputFile, "byte image%dHeight %d;\n", sequenceNumber, image.Bounds().Dy())

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

		os.Exit(8)
	}

	_, err = fmt.Fprintf(outputFile, "byte image%dBytes %d;\n", sequenceNumber, image.Bounds().Dy()*rowInBytes)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

		os.Exit(9)
	}

	_, err = fmt.Fprintln(outputFile)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

		os.Exit(10)
	}

	_, err = fmt.Fprintf(outputFile, "byte image%dData[] = {\n", sequenceNumber)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

		os.Exit(11)
	}

	// Now the actual image data

	for y := 0; y < image.Bounds().Dy(); y++ {
		_, err := fmt.Fprintf(outputFile, "\t")

		if err != nil {
			fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

			os.Exit(12)
		}

		somethingWritten := false

		for x := 0; x < rowInBytes; x++ {
			if somethingWritten {
				_, err := fmt.Fprintf(outputFile, ", 0b")

				if err != nil {
					fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

					os.Exit(13)
				}

				for i := 0; i < 8; i++ {
					pixel := color.Color(color.White)

					if x*8+i <= image.Bounds().Max.X {
						pixel = image.At(x*8+i, y)
					}

					bit := 0

					if pixel == color.Black {
						bit = 1
					}

					_, err := fmt.Fprintf(outputFile, "%d", bit)

					if err != nil {
						fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

						os.Exit(14)
					}
				}
			}

			somethingWritten = true
		}

		_, err = fmt.Fprintln(outputFile)

		if err != nil {
			fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

			os.Exit(15)
		}
	}

	// And the "footer"

	_, err = fmt.Fprintln(outputFile, "};")

	if err != nil {
		fmt.Fprintf(os.Stderr, "\t\t\tUnable to write C source %s: %s\n", filename, err)

		os.Exit(16)
	}
}

func findRowRanges(inputImage *image.RGBA) []IntRange {
	// Something to hold our results

	ranges := make([]IntRange, 0)

	// Get the bounds we have to work within

	imageBounds := inputImage.Bounds()

	startY, endY := imageBounds.Min.Y, imageBounds.Max.Y

	// Go through the rows, figuring things out

	currentStart := -1

	for y := startY; y <= endY; y++ {
		rowIsEmpty := isRowEmpty(inputImage, y)

		if rowIsEmpty {
			// If we don't have a range, nothing happens
			// If we have a range, we need to end it

			if currentStart != -1 {
				// We'll do a sanity check, and warn the user if things are off

				theRange := IntRange{currentStart, y - 1}

				if theRange.end-theRange.start >= 256 {
					fmt.Fprintf(os.Stderr, "Warning: Unbroken group of rows between %d and %d which is over 255 rows, skipping", theRange.start, theRange.end)
				} else {
					ranges = append(ranges, theRange)
				}

				currentStart = -1
			}
		} else {
			// If we don't have a range, start one
			// If we have a range, nothing happens

			if currentStart != -1 {
				currentStart = y
			}
		}
	}

	// Finish the last row, if there is one

	if currentStart != -1 {
		// We'll do a sanity check, and warn the user if things are off

		theRange := IntRange{currentStart, endY}

		if theRange.end-theRange.start >= 256 {
			fmt.Fprintf(os.Stderr, "Warning: Unbroken group of rows between %d and %d which is over 255 rows, skipping", theRange.start, theRange.end)
		} else {
			ranges = append(ranges, theRange)
		}
	}

	// Return those ranges

	return ranges
}

func findColumnRanges(inputImage *image.RGBA, startY int, endY int) []IntRange {
	// Something to hold our results

	ranges := make([]IntRange, 0)

	// Get the bounds we have to work within

	imageBounds := inputImage.Bounds()

	startX, endX := imageBounds.Min.X, imageBounds.Max.X

	// Go through the rows, figuring things out

	currentStart := -1

	for x := startX; x <= endX; x++ {
		columnIsEmpty := isColumnEmpty(inputImage, x, startY, endY)

		if columnIsEmpty {
			// If we don't have a range, nothing happens
			// If we have a range, we need to end it

			if currentStart != -1 {
				// We'll do a sanity check, and warn the user if things are off

				theRange := IntRange{currentStart, x - 1}

				if theRange.end-theRange.start >= 256 {
					fmt.Fprintf(os.Stderr, "Warning: Unbroken group of columns between %d and %d which is over 255 columns, skipping", theRange.start, theRange.end)
				} else {
					ranges = append(ranges, theRange)
				}

				currentStart = -1
			}
		} else {
			// If we don't have a range, start one
			// If we have a range, nothing happens

			if currentStart != -1 {
				currentStart = x
			}
		}
	}

	// Finish the last column, if there is one

	if currentStart != -1 {
		// We'll do a sanity check, and warn the user if things are off

		theRange := IntRange{currentStart, endX}

		if theRange.end-theRange.start >= 256 {
			fmt.Fprintf(os.Stderr, "Warning: Unbroken group of columns between %d and %d which is over 255 columns, skipping", theRange.start, theRange.end)
		} else {
			ranges = append(ranges, theRange)
		}
	}

	// Return those ranges

	return ranges
}

func isRowEmpty(inputImage *image.RGBA, y int) bool {
	imageBounds := inputImage.Bounds()

	if y < imageBounds.Min.Y || y > imageBounds.Max.Y {
		panic("Y value out of bounds")
	}

	for x := imageBounds.Min.X; x <= imageBounds.Max.X; x++ {
		if !colorIsWhite(inputImage.At(x, y)) {
			return false
		}
	}

	return true
}

func isColumnEmpty(inputImage *image.RGBA, x int, startY int, endY int) bool {
	for y := startY; y <= endY; y++ {
		if !colorIsWhite(inputImage.At(x, y)) {
			return false
		}
	}

	return true
}

func colorIsWhite(theColor color.Color) bool {
	// We expect grayscale images, so the fact that won't work well with colors is not important

	r, g, b, _ := theColor.RGBA()

	return (r + g + b) < (0x7FFF * 3)
}

func printHelp() {
	fmt.Println("TwoBitChunker by Michael Cook (http://www.foobarsoft.com)")
	fmt.Println()
	fmt.Printf("Usage: %s filename.img\n", os.Args[0])
	fmt.Println()
	fmt.Println("TwoBitChunker takes an image (preferably black & white) and finds individual chunks inside the image. It does this by scanning for rows and columns of white/clear data and using that information to generate simple bounding boxes. These sub-images are extracted, saved as PNGs and C source with one bit per pixel.")
	fmt.Println()
	fmt.Println("The input should be a GIF, PNG, or JPEG image. Pixels with an average RGB value over 127 will be considered black, all others white. Transparency will be ignored.")
	fmt.Println()
	fmt.Println("Outputs will be sequentially numbered PNGs and C source (i.e. 1.png and 1.c, 2.png and 2.c, etc). Inside the C files will be four variables: imageXWidth, imageXHeight, imageXBytes, and imageXData. Width and height will be bytes, data is a single-dimensional array of bytes containing the pixel data, padded to byte boundaries with 0s, in row order.")
}
