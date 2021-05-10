package internal

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

func GetArtifactZip(archiveDownloadURL, token string) (io.ReadCloser, error) {
	var bearer = "Bearer " + token
	req, err := http.NewRequest("GET", archiveDownloadURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", bearer)
	payloadResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact zip file: %w", err)
	}

	if payloadResp.StatusCode != http.StatusOK {
		payloadResp.Body.Close()
		return nil, fmt.Errorf("failed getting payload with status code: %d", payloadResp.StatusCode)
	}

	return payloadResp.Body, nil
}

func UnzipPayload(reader io.Reader, zipSize int) ([]byte, error) {
	buffer, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(buffer.Name())

	_, err = io.Copy(buffer, reader)
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(buffer, int64(zipSize))
	if err != nil {
		return nil, err
	}

	// Look for the event.json file inside the zip file
	for _, zipFile := range zipReader.File {
		fmt.Println("Reading file:", zipFile.Name)

		if zipFile.Name == "event.json" {
			unzippedFileBytes, err := readZipFile(zipFile)
			if err != nil {
				return nil, err
			}
			return unzippedFileBytes, nil
		}
	}
	return nil, fmt.Errorf("no payload with the name event.json found in zip")
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}
