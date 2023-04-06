package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var flagURL string
var flagPathsToCheck arrayFlags
var flagPathToCopy string

func init() {
	flag.Var(&flagPathsToCheck, "check", "paths to check before copying (multiple paths accepted)")
	flag.StringVar(&flagURL, "url", "https://github.com/schollz/portedplugins/releases/download/v0.4.5/PortedPlugins-Linux.zip", "url to download zip")
	flag.StringVar(&flagPathToCopy, "to", ".", "path to copy files to")
}

func main() {
	flag.Parse()

	run()
}

func run() (err error) {
	os.Mkdir("ignore", 0644)
	defer os.RemoveAll("ignore")
	err = downloadAndUnzip()
	if err != nil {
		fmt.Println(err)
		return
	}

	files_that_exist := make(map[string]string)
	for _, p := range flagPathsToCheck {
		err = filepath.Walk(p, func(pp string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Println(err)
				return err
			}
			if !info.IsDir() {
				ppAbs, _ := filepath.Abs(pp)
				ppAbs = filepath.ToSlash(ppAbs)
				// ignore things in "ignore" directory
				if !strings.Contains(ppAbs, "/ignore/") {
					files_that_exist[filepath.Base(pp)] = pp
				}
			}
			return nil
		})
		if err != nil {
			fmt.Println(err)
		}
	}
	// fmt.Printf("files_that_exist: %+v\n", files_that_exist)

	// move files in ignore if they don't already exist
	os.MkdirAll(flagPathToCopy, 0644)
	err = filepath.Walk("ignore", func(pp string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		if !info.IsDir() {
			ppBase := filepath.Base(pp)
			if _, ok := files_that_exist[ppBase]; ok {
				fmt.Printf("skipping %s, already exists in %s\n", pp, files_that_exist[ppBase])
			} else {
				newFile := path.Join(flagPathToCopy, ppBase)
				fmt.Printf("copying %s to %s\n", pp, newFile)
				err2 := copyFile(pp, newFile)
				if err2 != nil {
					fmt.Printf("error: %s", err2.Error())
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}

	return
}

func downloadAndUnzip() (err error) {
	u, err := url.Parse(flagURL)
	if err != nil {
		fmt.Println(err)
		return
	}
	fname := path.Base(u.Path)

	// download file
	fmt.Printf("downloading %s...\n", flagURL)
	out, err := os.Create(path.Join("ignore", fname))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer out.Close()

	resp, err := http.Get(flagURL)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("wrote %d bytes\n", n)

	// unzip file
	err = unzip(path.Join("ignore", fname), "ignore")
	return
}

// func ex(cm string) (err error) {
// 	fmt.Println(cm)
// 	cc := strings.Fields(cm)
// 	cmd := exec.Command(cc[0], cc[1:]...)
// 	stdoutStderr, err := cmd.CombinedOutput()
// 	if err != nil {
// 		fmt.Println(cm)
// 		fmt.Printf("%s\n", stdoutStderr)
// 	}
// 	return
// }

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
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

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			fmt.Printf("unzipping %s\n", path)
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
			return err
		}
	}

	return nil
}

// copyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func copyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
