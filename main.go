package main

import (
	_ "embed"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
)

//go:embed license_text
var licenseText string

var pathFlag string
var recursiveFlag bool
var fileEndingsFlag string
var scrapeLicenseFlag string
var dryRunFlag bool
var textFileFlag string
var updateFlag bool
var purgeFlag bool

func main() {
	flag.StringVar(&pathFlag, "d", ".", "Path to the directory to check")
	flag.BoolVar(&recursiveFlag, "r", false, "Check the directory recursively")
	flag.StringVar(&fileEndingsFlag, "e", "", "Comma separated list of file endings to check")
	flag.StringVar(&scrapeLicenseFlag, "s", "", "URL to scrape the license from")
	flag.BoolVar(&dryRunFlag, "v", false, "Perform a dry run to check which files would be checked")
	flag.StringVar(&textFileFlag, "t", "", "Path to a text file containing the license text")
	flag.BoolVar(&updateFlag, "u", false, "Update the license text")
	flag.BoolVar(&purgeFlag, "p", false, "Purge the license text")
	flag.Parse()

	fileTypes := strings.Split(fileEndingsFlag, ",")
	path, ok := processPath(pathFlag)
	if !ok {
		println("Path not ok. Please enter a valid path.")
		return
	}
	err := setLicenseText()
	if err != nil {
		println("Something went wrong setting the license text.")
		panic(err)
	}

	changedFiles, failedFiles, err := checkDirectory(path, fileTypes)
	if err != nil {
		println("Something bad happened.")
		println(err)
		return
	}
	println(licenseText)
	println("Changed ", len(changedFiles), " files")
	println(strings.Join(changedFiles, "\n"))
	println()
	println("Failed to Change ", len(failedFiles), " files")
	println(strings.Join(failedFiles, "\n"))
	if dryRunFlag {
		println()
		println("This was a dry run. No files were changed.")
	}
}

func checkDirectory(path string, fileTypes []string) ([]string, []string, error) {
	println("Checking Directory: ", path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return []string{}, []string{}, err
	}
	changedFileNames := []string{}
	failedFileNames := []string{}
	for _, file := range entries {
		filePath := path + file.Name()

		if file.IsDir() {
			if recursiveFlag {
				changedFiles, failedFiles, err := checkDirectory(filePath, fileTypes)
				if err == nil {
					changedFileNames = append(changedFileNames, changedFiles...)
					failedFileNames = append(failedFileNames, failedFiles...)
				}
			}
			continue
		}
		if len(fileTypes) > 0 {
			var ok bool = false
			for _, typ := range fileTypes {
				if strings.HasSuffix(file.Name(), typ) {
					ok = true
					break
				}
			}
			if !ok {
				// We don't add these to the failed files bc filtering out the files by ending is intended behavior
				continue
			}
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			failedFileNames = append(failedFileNames, filePath)
			continue
		}
		var newContent []byte
		if purgeFlag {
			var exists bool
			newContent, exists = purgeLicense(content)
			if !exists {
				continue
			}
		} else {
			var exists bool
			newContent, exists = addLicense(content)
			if exists {
				continue
			}
		}
		if dryRunFlag {
			f, err := os.OpenFile(filePath, os.O_RDWR, 0666)
			f.Close()
			if err != nil {
				failedFileNames = append(failedFileNames, filePath)
				continue
			}
		} else {
			err = os.WriteFile(filePath, newContent, 0666)
			if err != nil {
				failedFileNames = append(failedFileNames, filePath)
				continue
			}
		}
		changedFileNames = append(changedFileNames, filePath)
	}
	return changedFileNames, failedFileNames, nil
}

func addLicense(content []byte) ([]byte, bool) {
	exists := strings.HasPrefix(string(content), licenseText)
	if exists {
		return content, exists
	}
	if !updateFlag {
		return append([]byte(licenseText), content...), false
	}

	license_end_marker := "********************************************************/\n"
	splitContent := strings.SplitN(string(content), license_end_marker, 2)
	if len(splitContent) < 2 {
		return append([]byte(licenseText), content...), false
	}
	contentNoLicense := splitContent[1]
	return append([]byte(licenseText), []byte(contentNoLicense)...), false
}

func purgeLicense(content []byte) ([]byte, bool) {
	license_end_marker := "********************************************************/\n"
	splitContent := strings.SplitN(string(content), license_end_marker, 2)
	if len(splitContent) < 2 {
		return content, false
	}
	contentNoLicense := splitContent[1]
	return []byte(contentNoLicense), true
}

func processPath(rawPath string) (string, bool) {
	stat, err := os.Stat(rawPath)
	if err != nil || !stat.IsDir() {
		return "", false
	}

	path := rawPath
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	return path, true
}

func setLicenseText() error {
	if textFileFlag != "" && scrapeLicenseFlag != "" {
		return errors.New("Please provide either a file path or a scrape url")
	}
	if scrapeLicenseFlag != "" {
		res, err := http.Get(scrapeLicenseFlag)
		if err != nil {
			return err
		}
		text, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		licenseText = string(text)
	} else if textFileFlag != "" {
		contents, err := os.ReadFile(textFileFlag)
		if err != nil {
			return err
		}
		licenseText = string(contents)
	}
	return nil
}
