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

func main() {
	pathFlag := flag.String("d", ".", "Path to the directory to check")
	recursiveFlag := flag.Bool("r", false, "Check the directory recursively")
	fileEndingsFlag := flag.String("e", "", "Comma separated list of file endings to check")
	scrapeLicenseFlag := flag.String("s", "", "URL to scrape the license from")
	dryRunFlag := flag.Bool("v", false, "Perform a dry run to check which files would be checked")
	textFileFlag := flag.String("t", "", "Path to a text file containing the license text")
	updateFlag := flag.Bool("u", false, "Update the license text")
	flag.Parse()

	fileTypes := strings.Split(*fileEndingsFlag, ",")
	path, ok := processPath(*pathFlag)
	if !ok {
		println("Path not ok. Please enter a valid path.")
		return
	}
	err := setLicenseText(*scrapeLicenseFlag, *textFileFlag)
	if err != nil {
		println("Something went wrong setting the license text.")
		panic(err)
	}

	changedFiles, failedFiles, err := checkDirectory(path, fileTypes, *recursiveFlag, *dryRunFlag, *updateFlag)
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
	if *dryRunFlag {
		println()
		println("This was a dry run. No files were changed.")
	}
}

func checkDirectory(path string, fileTypes []string, recursively bool, dryrun bool, update bool) ([]string, []string, error) {
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
			if recursively {
				changedFiles, failedFiles, err := checkDirectory(filePath, fileTypes, recursively, dryrun, update)
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

		newContent, exists := addLicense(content, update)
		if exists {
			// If the license notifier already exists, don't add it
			continue
		}

		if dryrun {
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

func addLicense(content []byte, update bool) ([]byte, bool) {
	exists := strings.HasPrefix(string(content), licenseText)
	if exists {
		return content, exists
	}
	if !update {
		return append([]byte(licenseText), content...), false
	}

	license_end_marker := "********************************************************/\n"
	contentNoLicense := strings.SplitN(string(content), license_end_marker, 2)[1]
	return append([]byte(licenseText), []byte(contentNoLicense)...), false
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

func setLicenseText(scrapeUrl string, filePath string) error {
	if filePath != "" && scrapeUrl != "" {
		return errors.New("Please provide either a file path or a scrape url")
	}
	if scrapeUrl != "" {

		res, err := http.Get(scrapeUrl)
		if err != nil {
			return err
		}
		text, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		licenseText = string(text)
	} else if filePath != "" {
		contents, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		licenseText = string(contents)
	}
	return nil
}
