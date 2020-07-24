package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"github.com/gookit/color"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

func main() {
	sukkit := "  ____        _    _    _ _   \n / ___| _   _| | _| | _(_) |_ \n \\___ \\| | | | |/ / |/ / | __|\n  ___) | |_| |   <|   <| | |_ \n |____/ \\__,_|_|\\_\\_|\\_\\_|\\__|\n                              "
	color.LightYellow.Println(sukkit)
	time.Sleep(500 * time.Millisecond)

	color.LightYellow.Println("Sukkit. The solar powered server software.")

	color.LightYellow.Println("Press enter to start setup.")
	fmt.Scanln()

	color.LightCyan.Println("Downloading files...")
	files := getFiles()
	var wg sync.WaitGroup
	wg.Add(len(files))

	// Create new progress container instance
	p := mpb.New(mpb.WithWaitGroup(&wg), mpb.WithWidth(60))
	for filename, url := range files {
		go func(filename string, url string) {
			defer wg.Done()
			err := downloadFile(p, url, filename)
			if err != nil {
				panic(err)
			}
		}(filename, url)
	}
	// Wait for all bars to complete
	p.Wait()

	color.LightCyan.Println("Unleashing the power...")
	if runtime.GOOS == "windows" {
		unzip("php", ".")
	} else {
		extractTarGz("php", ".")
	}

	color.LightCyan.Println("Feeding the leftovers to dogs...")
	if err := deleteFile("php"); err != nil {
		panic(err)
	}

	color.BgGreen.Println("Installation complete!")

	time.Sleep(3 * time.Second)
	color.Gray.Println("Press enter to continue...")
	fmt.Scanln()
}

// Get the required files based on the user's OS
func getFiles() map[string]string {
	suffix := "sh"
	php := "https://jenkins.pmmp.io/job/PHP-7.3-Aggregate/lastSuccessfulBuild/artifact/PHP-7.3-Linux-x86_64.tar.gz"
	if runtime.GOOS == "windows" {
		suffix = "ps1"
		php = "https://jenkins.pmmp.io/job/PHP-7.3-Aggregate/lastSuccessfulBuild/artifact/PHP-7.3-Windows-x64.zip"
	}
	if runtime.GOOS == "darwin" {
		php = "https://jenkins.pmmp.io/job/PHP-7.3-Aggregate/lastSuccessfulBuild/artifact/PHP-7.3-MacOS-x86_64.tar.gz"
	}
	m := make(map[string]string)
	m["php"] = php
	m["PocketMine-MP.phar"] = "https://github.com/pmmp/PocketMine-MP/releases/download/3.14.2/PocketMine-MP.phar"
	m["start."+suffix] = "https://github.com/pmmp/PocketMine-MP/releases/download/3.14.2/start." + suffix
	return m
}

// Download a file from a url into given filename.
func downloadFile(p *mpb.Progress, url string, filename string) error {
	// Create file
	out, err := os.Create(filename + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()

	// the Header "Content-Length" will let us know
	// the total file size to download
	size, _ := strconv.Atoi(resp.Header.Get("Content-Length"))

	// Create new bar with filesize
	bar := p.AddBar(int64(size),
		mpb.PrependDecorators(
			decor.Percentage(),
		),
		mpb.AppendDecorators(
			decor.Name(filename, decor.WC{W: len(filename) + 1, C: decor.DidentRight}),
		),
	)

	// Create a proxy reader for the bar
	proxyReader := bar.ProxyReader(resp.Body)
	defer proxyReader.Close()

	// Start the copy action with our proxy reader
	_, err = io.Copy(out, proxyReader)
	if err != nil {
		return err
	}

	// Close it before renaming to prevent file in use error
	out.Close()

	// The bar uses the same line so print a new line once it's finished downloading
	fmt.Println()

	if err := os.Rename(filename+".tmp", filename); err != nil {
		return err
	}
	return nil
}

// Erase a file from disk
func deleteFile(file string) error {
	return os.Remove(file)
}

// Unzip a zip archive
func unzip(archive string, dest string) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			panic(err)
		}
	}
}

func extractTarGz(archive string, dest string) {
	r, err := os.Open(archive)
	if err != nil {
		panic(err)
	}

	uncompressedStream, err := gzip.NewReader(r)
	if err != nil {
		panic(err)
	}

	tarReader := tar.NewReader(uncompressedStream)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			panic(err)
		}

		if header.Typeflag == tar.TypeDir {
			if err := os.Mkdir(header.Name, 0755); err != nil {
				panic(err)
			}
		} else {
			outFile, err := os.Create(header.Name)
			if err != nil {
				panic(err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				panic(err)
			}
			outFile.Close()
		}
	}
}
